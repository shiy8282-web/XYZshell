package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
	"xyzshell/pkg/serialclient"
	"xyzshell/pkg/sessionlog"
	"xyzshell/pkg/sshclient"
	"xyzshell/pkg/telnetclient"
	"xyzshell/pkg/xyzshell"
)

type terminalSession interface {
	Input() io.WriteCloser
	Output() io.Reader
	Resize(rows, columns uint) error
	Close() error
}

func main() {
	a := app.NewWithID("io.xyzshell.desktop")
	w := a.NewWindow(fmt.Sprintf("XYZshell v%s", xyzshell.Version))
	w.Resize(fyne.NewSize(900, 620))

	protocol := widget.NewSelect([]string{"SSH", "Telnet", "Serial"}, nil)
	protocol.SetSelected("SSH")
	host := widget.NewEntry()
	host.SetPlaceHolder("example.com")
	user := widget.NewEntry()
	user.SetPlaceHolder("SSH 用户名")
	user.Wrapping = fyne.TextWrapOff
	user.Scroll = fyne.ScrollNone
	port := widget.NewEntry()
	port.SetText("22")
	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("密码")
	serialPort := widget.NewSelectEntry(nil)
	serialPort.SetPlaceHolder("COM3 或 /dev/ttyUSB0")
	baud := widget.NewEntry()
	baud.SetText("9600")
	dataBits := widget.NewSelect([]string{"5", "6", "7", "8"}, nil)
	dataBits.SetSelected("8")
	parity := widget.NewSelect([]string{"None", "Even", "Odd"}, nil)
	parity.SetSelected("None")
	stopBits := widget.NewSelect([]string{"1", "2"}, nil)
	stopBits.SetSelected("1")
	historySelect := widget.NewSelect(nil, nil)
	history := []xyzshell.ConnectionProfile{}
	status := widget.NewLabel("未连接")
	cipherStatus := widget.NewLabel("加密方式：未连接")
	recordLog := widget.NewCheck("下次连接记录终端输入/输出（可能包含密码）", nil)
	recordLog.SetChecked(false)
	termView := terminal.New()
	resizeEvents := make(chan terminal.Config, 1)
	termView.AddListener(resizeEvents)

	var resizeMu sync.Mutex
	var lastSize terminal.Config
	var resizeSession terminalSession
	go func() {
		for size := range resizeEvents {
			resizeMu.Lock()
			lastSize = size
			s := resizeSession
			resizeMu.Unlock()
			if s != nil {
				if err := s.Resize(size.Rows, size.Columns); err != nil {
					fyne.Do(func() { status.SetText("终端尺寸同步失败: " + err.Error()) })
				}
			}
		}
	}()

	inlineField := func(label string, control fyne.CanvasObject, width float32) fyne.CanvasObject {
		labelWidget := widget.NewLabel(label)
		if width > 0 {
			fixedControl := container.NewGridWrap(fyne.NewSize(width, control.MinSize().Height), control)
			return container.NewHBox(labelWidget, fixedControl, layout.NewSpacer())
		}
		return container.NewHBox(labelWidget, control)
	}
	hostField := inlineField("主机", host, 150)
	portField := inlineField("端口", port, 72)
	userField := inlineField("用户名", user, 180)
	passwordField := inlineField("密码", password, 180)
	serialField := inlineField("串口", serialPort, 92)
	baudField := inlineField("波特率", baud, 72)
	dataBitsField := inlineField("数据位", dataBits, 64)
	parityField := inlineField("校验位", parity, 76)
	stopBitsField := inlineField("停止位", stopBits, 60)
	historyField := inlineField("最近连接", historySelect, 150)
	refreshPorts := widget.NewButton("刷新串口", func() {
		ports, err := serialclient.ListPorts()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		serialPort.SetOptions(ports)
		if len(ports) > 0 && serialPort.Text == "" {
			serialPort.SetText(ports[0])
		}
	})
	protocolField := inlineField("连接方式", protocol, 92)
	connectionFields := container.NewHBox(
		protocolField, hostField, portField, historyField, layout.NewSpacer())
	authFields := container.NewHBox(userField, passwordField, recordLog, layout.NewSpacer())
	serialFields := container.NewHBox(
		serialField, refreshPorts, baudField, dataBitsField, parityField, stopBitsField, layout.NewSpacer())
	serialFields.Hide()
	setVisible := func(obj fyne.CanvasObject, visible bool) {
		if visible {
			obj.Show()
		} else {
			obj.Hide()
		}
	}
	updateFields := func(value string) {
		isSerial := value == "Serial"
		isSSH := value == "SSH"
		setVisible(hostField, !isSerial)
		setVisible(portField, !isSerial)
		setVisible(userField, isSSH)
		setVisible(passwordField, isSSH)
		setVisible(serialField, isSerial)
		setVisible(serialFields, isSerial)
		if value == "Telnet" && port.Text == "22" {
			port.SetText("23")
		} else if value == "SSH" && port.Text == "23" {
			port.SetText("22")
		}
	}
	protocol.OnChanged = updateFields
	updateFields(protocol.Selected)

	historySelect.OnChanged = func(label string) {
		for _, profile := range history {
			if profile.DisplayName() != label {
				continue
			}
			protocol.SetSelected(profile.Protocol)
			host.SetText(profile.Host)
			user.SetText(profile.User)
			if profile.Port != 0 {
				port.SetText(strconv.Itoa(int(profile.Port)))
			}
			serialPort.SetText(profile.SerialPort)
			if profile.BaudRate != 0 {
				baud.SetText(strconv.Itoa(profile.BaudRate))
			}
			if profile.DataBits != 0 {
				dataBits.SetSelected(strconv.Itoa(profile.DataBits))
			}
			if profile.Parity != "" {
				parity.SetSelected(profile.Parity)
			}
			if profile.StopBits != 0 {
				stopBits.SetSelected(strconv.Itoa(profile.StopBits))
			}
			password.SetText("")
			return
		}
	}
	refreshHistory := func(profiles []xyzshell.ConnectionProfile) {
		history = profiles
		options := make([]string, 0, len(profiles))
		for _, profile := range profiles {
			options = append(options, profile.DisplayName())
		}
		historySelect.SetOptions(options)
		if len(options) == 0 {
			historySelect.ClearSelected()
		}
	}
	if profiles, err := xyzshell.LoadConnectionHistory(); err != nil {
		status.SetText("连接历史读取失败: " + err.Error())
	} else {
		refreshHistory(profiles)
	}

	var current terminalSession
	connecting := false
	var connectButton, disconnectButton *widget.Button

	connectButton = widget.NewButton("连接", func() {
		if connecting || current != nil {
			return
		}
		selected := protocol.Selected
		var cfg xyzshell.Config
		var serialCfg serialclient.Config
		var profile xyzshell.ConnectionProfile
		var target string
		var networkPort uint16
		switch selected {
		case "SSH", "Telnet":
			portValue, err := strconv.ParseUint(strings.TrimSpace(port.Text), 10, 16)
			if err != nil || portValue == 0 {
				dialog.ShowError(fmt.Errorf("端口必须是 1 到 65535 之间的数字"), w)
				return
			}
			networkPort = uint16(portValue)
			target = strings.TrimSpace(host.Text)
			if selected == "SSH" {
				cfg = xyzshell.Config{Host: target, User: strings.TrimSpace(user.Text), Port: networkPort}
				if err := cfg.Validate(); err != nil {
					dialog.ShowError(err, w)
					return
				}
				profile = xyzshell.ConnectionProfile{Protocol: "SSH", Host: cfg.Host, Port: cfg.Port, User: cfg.User}
			} else {
				if target == "" || strings.ContainsAny(target, " \t\r\n/\\") {
					dialog.ShowError(fmt.Errorf("请输入有效的 Telnet 主机名或 IP 地址"), w)
					return
				}
				profile = xyzshell.ConnectionProfile{Protocol: "Telnet", Host: target, Port: networkPort}
			}
		case "Serial":
			target = strings.TrimSpace(serialPort.Text)
			if target == "" {
				dialog.ShowError(fmt.Errorf("请选择或输入串口名称"), w)
				return
			}
			rate, err := strconv.Atoi(strings.TrimSpace(baud.Text))
			if err != nil {
				dialog.ShowError(fmt.Errorf("波特率必须是数字"), w)
				return
			}
			bits, _ := strconv.Atoi(dataBits.Selected)
			stops, _ := strconv.Atoi(stopBits.Selected)
			serialCfg = serialclient.Config{
				Port: target, BaudRate: rate, DataBits: bits,
				Parity: parity.Selected, StopBits: stops,
			}
			profile = xyzshell.ConnectionProfile{
				Protocol: "Serial", SerialPort: target, BaudRate: rate,
				DataBits: bits, Parity: parity.Selected, StopBits: stops,
			}
		default:
			dialog.ShowError(fmt.Errorf("请选择连接方式"), w)
			return
		}

		secret := password.Text
		password.SetText("")
		saveLog := recordLog.Checked
		connecting = true
		connectButton.Disable()
		status.SetText("正在连接…")
		go func() {
			var s terminalSession
			var err error
			switch selected {
			case "SSH":
				s, err = sshclient.Connect(context.Background(), cfg, secret, func(host, fingerprint string) bool {
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
			case "Telnet":
				s, err = telnetclient.Connect(target, networkPort)
			case "Serial":
				s, err = serialclient.Connect(serialCfg)
			}
			fyne.Do(func() {
				connecting = false
				if err != nil {
					connectButton.Enable()
					status.SetText("连接失败")
					if selected == "SSH" && strings.TrimSpace(cfg.User) == "" && strings.Contains(strings.ToLower(err.Error()), "auth") {
						dialog.ShowError(fmt.Errorf("设备未接受 SSH 免凭据登录。SSH 通常需要在建立终端前提供用户名；若设备要在终端内提示账号密码，请选择 Telnet 或串口连接。\n\n详细信息：%w", err), w)
						return
					}
					dialog.ShowError(err, w)
					return
				}
				var recorder *sessionlog.Recorder
				if saveLog {
					recorder, err = sessionlog.New(profile)
					if err != nil {
						_ = s.Close()
						connectButton.Enable()
						status.SetText("日志文件创建失败")
						dialog.ShowError(err, w)
						return
					}
				}
				current = s
				connectButton.Disable()
				disconnectButton.Enable()
				switch selected {
				case "Serial":
					status.SetText("已连接串口 " + target)
					cipherStatus.SetText("加密方式：串口连接（不使用网络加密算法）")
				case "Telnet":
					status.SetText(fmt.Sprintf("已连接到 %s:%d（明文）", target, networkPort))
					cipherStatus.SetText("加密方式：无（Telnet 明文连接）")
				default:
					status.SetText("已连接到 " + cfg.Address())
					if details, ok := s.(interface{ EncryptionSummary() string }); ok {
						cipherStatus.SetText("加密方式：" + details.EncryptionSummary())
					} else {
						cipherStatus.SetText("加密方式：SSH（协商信息不可用）")
					}
				}
				if profiles, historyErr := xyzshell.RememberConnection(profile); historyErr != nil {
					dialog.ShowError(fmt.Errorf("连接已成功，但无法保存历史记录：%w", historyErr), w)
				} else {
					refreshHistory(profiles)
				}
				resizeMu.Lock()
				resizeSession = s
				initialSize := lastSize
				resizeMu.Unlock()
				_ = s.Resize(initialSize.Rows, initialSize.Columns)

				input := s.Input()
				var output io.Reader = s.Output()
				if recorder != nil {
					input = recorder.WrapInput(input)
					output = recorder.WrapOutput(output)
					status.SetText(status.Text + "；日志：" + recorder.Path())
				}
				output = newInterfaceColorReader(output)
				go func() {
					_ = termView.RunWithConnection(input, output)
					resizeMu.Lock()
					if resizeSession == s {
						resizeSession = nil
					}
					resizeMu.Unlock()
					_ = s.Close()
					if recorder != nil {
						_ = recorder.Close()
					}
					fyne.Do(func() {
						if current == s {
							current = nil
							connectButton.Enable()
							disconnectButton.Disable()
							status.SetText("连接已关闭")
							cipherStatus.SetText("加密方式：未连接")
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
		s := current
		current = nil
		resizeMu.Lock()
		if resizeSession == s {
			resizeSession = nil
		}
		resizeMu.Unlock()
		disconnectButton.Disable()
		connectButton.Enable()
		status.SetText("正在断开…")
		go func() {
			_ = s.Close()
			fyne.Do(func() {
				status.SetText("已断开")
				cipherStatus.SetText("加密方式：未连接")
			})
		}()
	})
	disconnectButton.Disable()

	algorithms := widget.NewButton("SSH 加密算法", func() {
		alg := xyzshell.Algorithms()
		legacy := xyzshell.InsecureAlgorithms()
		body := fmt.Sprintf("当前安全默认值（按优先顺序）\n\n密钥交换\n%s\n\n主机密钥\n%s\n\n加密\n%s\n\nMAC\n%s\n\n已实现但默认排除的旧算法\n%s\n%s\n%s\n%s",
			strings.Join(alg.KeyExchanges, "\n"), strings.Join(alg.HostKeys, "\n"), strings.Join(alg.Ciphers, "\n"), strings.Join(alg.MACs, "\n"),
			strings.Join(legacy.KeyExchanges, "\n"), strings.Join(legacy.HostKeys, "\n"), strings.Join(legacy.Ciphers, "\n"), strings.Join(legacy.MACs, "\n"))
		dialog.ShowInformation("SSH 算法能力", body, w)
	})

	top := container.NewVBox(connectionFields, authFields, serialFields)
	header := container.NewBorder(nil, nil, widget.NewLabelWithStyle("XYZshell", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(algorithms, connectButton, disconnectButton), status)
	w.SetContent(container.NewBorder(container.NewVBox(header, top), cipherStatus, nil, nil, termView))
	w.SetOnClosed(func() {
		termView.RemoveListener(resizeEvents)
		if current != nil {
			_ = current.Close()
		}
	})
	w.ShowAndRun()
}

