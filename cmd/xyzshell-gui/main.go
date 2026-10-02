package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
	"xyzshell/pkg/sshclient"
	"xyzshell/pkg/xyzshell"
)

func main() {
	a := app.NewWithID("io.xyzshell.desktop")
	w := a.NewWindow("XYZshell")
	w.Resize(fyne.NewSize(1120, 760))

	host := widget.NewEntry()
	host.SetPlaceHolder("example.com")
	user := widget.NewEntry()
	user.SetPlaceHolder("用户名")
	port := widget.NewEntry()
	port.SetText("22")
	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("密码仅用于本次连接")
	status := widget.NewLabel("未连接")
	termView := terminal.New()
	resizeEvents := make(chan terminal.Config, 1)
	termView.AddListener(resizeEvents)
	var resizeMu sync.Mutex
	var lastSize terminal.Config
	var resizeClient *sshclient.Client
	go func() {
		for size := range resizeEvents {
			resizeMu.Lock()
			lastSize = size
			client := resizeClient
			resizeMu.Unlock()
			if client != nil {
				if err := client.Resize(size.Rows, size.Columns); err != nil {
					fyne.Do(func() { status.SetText("终端尺寸同步失败: " + err.Error()) })
				}
			}
		}
	}()
	var current *sshclient.Client
	connecting := false
	var connectButton, disconnectButton *widget.Button

	connectButton = widget.NewButton("连接", func() {
		if connecting || current != nil {
			return
		}
		portValue, err := strconv.ParseUint(strings.TrimSpace(port.Text), 10, 16)
		if err != nil || portValue == 0 {
			dialog.ShowError(fmt.Errorf("端口必须是 1 到 65535 之间的数字"), w)
			return
		}
		cfg := xyzshell.Config{Host: strings.TrimSpace(host.Text), User: strings.TrimSpace(user.Text), Port: uint16(portValue)}
		if err := cfg.Validate(); err != nil {
			dialog.ShowError(err, w)
			return
		}

		secret := password.Text
		password.SetText("")
		connecting = true
		connectButton.Disable()
		status.SetText("正在连接…")
		go func() {
			client, err := sshclient.Connect(context.Background(), cfg, secret, func(host, fingerprint string) bool {
				decision := make(chan bool, 1)
				fyne.Do(func() {
					dialog.ShowConfirm("验证 SSH 主机密钥",
						fmt.Sprintf("服务器：%s\nSHA256 指纹：\n%s\n\n仅当你确认这是目标服务器时才接受。", host, fingerprint),
						func(ok bool) { decision <- ok }, w)
				})
				select {
				case ok := <-decision:
					return ok
				case <-time.After(2 * time.Minute):
					return false
				}
			})
			fyne.Do(func() {
				connecting = false
				if err != nil {
					connectButton.Enable()
					status.SetText("连接失败")
					dialog.ShowError(err, w)
					return
				}
				current = client
				connectButton.Disable()
				disconnectButton.Enable()
				status.SetText("已连接到 " + cfg.Address())

				resizeMu.Lock()
				resizeClient = client
				initialSize := lastSize
				resizeMu.Unlock()
				_ = client.Resize(initialSize.Rows, initialSize.Columns)
				go func() {
					_ = termView.RunWithConnection(client.Input(), client.Output())
					resizeMu.Lock()
					if resizeClient == client {
						resizeClient = nil
					}
					resizeMu.Unlock()
					_ = client.Close()
					fyne.Do(func() {
						if current == client {
							current = nil
							connectButton.Enable()
							disconnectButton.Disable()
							status.SetText("连接已关闭")
						}
					})
				}()
			})
		}()
	})
	disconnectButton = widget.NewButton("断开", func() {
		if current == nil {
			return
		}
		client := current
		current = nil
		resizeMu.Lock()
		if resizeClient == client {
			resizeClient = nil
		}
		resizeMu.Unlock()
		disconnectButton.Disable()
		connectButton.Enable()
		status.SetText("正在断开…")
		go func() {
			_ = client.Close()
			fyne.Do(func() { status.SetText("已断开") })
		}()
	})
	disconnectButton.Disable()

	algorithms := widget.NewButton("加密算法", func() {
		alg := xyzshell.Algorithms()
		legacy := xyzshell.InsecureAlgorithms()
		body := fmt.Sprintf("当前安全默认值（按优先顺序）\n\n密钥交换\n%s\n\n主机密钥\n%s\n\n加密\n%s\n\nMAC\n%s\n\n已实现但默认排除的旧算法\n%s\n%s\n%s\n%s",
			strings.Join(alg.KeyExchanges, "\n"), strings.Join(alg.HostKeys, "\n"), strings.Join(alg.Ciphers, "\n"), strings.Join(alg.MACs, "\n"),
			strings.Join(legacy.KeyExchanges, "\n"), strings.Join(legacy.HostKeys, "\n"), strings.Join(legacy.Ciphers, "\n"), strings.Join(legacy.MACs, "\n"))
		dialog.ShowInformation("SSH 算法能力", body, w)
	})
	field := func(label string, item fyne.CanvasObject) fyne.CanvasObject {
		return container.NewVBox(widget.NewLabel(label), item)
	}
	form := container.NewGridWithColumns(4,
		field("主机", host), field("用户名", user), field("端口", port), field("密码", password))
	header := container.NewBorder(nil, nil, widget.NewLabelWithStyle("XYZshell", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(algorithms, connectButton, disconnectButton), status)
	w.SetContent(container.NewBorder(container.NewVBox(header, form), nil, nil, nil, termView))
	w.SetOnClosed(func() {
		termView.RemoveListener(resizeEvents)
		if current != nil {
			_ = current.Close()
		}
	})
	w.ShowAndRun()
}
