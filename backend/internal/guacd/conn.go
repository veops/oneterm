package guacd

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/tunneling"
	"github.com/veops/oneterm/pkg/config"
	"github.com/veops/oneterm/pkg/logger"
)

const (
	VERSION          = "VERSION_1_5_0"
	RECORDING_PATH   = "/replay"
	CREATE_RECORDING = "true"
	IGNORE_CERT      = "true"
)

// File transfer parameters
const (
	DRIVE_ENABLE           = "enable-drive"
	DRIVE_PATH             = "drive-path"
	DRIVE_CREATE_PATH      = "create-drive-path"
	DRIVE_DISABLE_UPLOAD   = "disable-upload"
	DRIVE_DISABLE_DOWNLOAD = "disable-download"
	DRIVE_NAME             = "drive-name"
)

type Configuration struct {
	Protocol   string
	Parameters map[string]string
}

func NewConfiguration() (config *Configuration) {
	config = &Configuration{}
	config.Parameters = make(map[string]string)
	return config
}

type Tunnel struct {
	SessionId       string
	ConnectionId    string
	conn            net.Conn
	reader          *bufio.Reader
	writer          *bufio.Writer
	Config          *Configuration
	gw              *tunneling.GatewayTunnel
	transferManager *FileTransferManager
	drivePath       string
}

func NewTunnel(connectionId, sessionId string, w, h, dpi int, protocol string, asset *model.Asset, account *model.Account, gateway *model.Gateway) (t *Tunnel, err error) {
	var hostPort string
	if strings.Contains(config.Cfg.Guacd.Host, ":") {
		// IPv6 address
		hostPort = fmt.Sprintf("[%s]:%d", config.Cfg.Guacd.Host, config.Cfg.Guacd.Port)
	} else {
		// IPv4 address or hostname
		hostPort = fmt.Sprintf("%s:%d", config.Cfg.Guacd.Host, config.Cfg.Guacd.Port)
	}
	conn, err := net.DialTimeout("tcp", hostPort, time.Second*3)
	if err != nil {
		return
	}
	ss := strings.Split(protocol, ":")
	protocol, port := ss[0], ss[1]
	cfg := model.GlobalConfig.Load()
	t = &Tunnel{
		conn:            conn,
		reader:          bufio.NewReader(conn),
		writer:          bufio.NewWriter(conn),
		ConnectionId:    connectionId,
		transferManager: DefaultFileTransferManager,
		Config: &Configuration{
			Protocol: protocol,
			Parameters: lo.TernaryF(
				connectionId == "",
				func() map[string]string {
					return map[string]string{
						"version":               VERSION,
						"client-name":           "OneTerm",
						"recording-path":        RECORDING_PATH,
						"create-recording-path": CREATE_RECORDING,
						"ignore-cert":           IGNORE_CERT,
						"width":                 cast.ToString(w),
						"height":                cast.ToString(h),
						"dpi":                   cast.ToString(dpi),
						"scheme":                protocol,
						"hostname":              asset.Ip,
						"port":                  port,
						"username":              account.Account,
						"password":              account.Password,
						"disable-copy":          cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), !cfg.RdpConfig.Copy, !cfg.VncConfig.Copy)),
						"disable-paste":         cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), !cfg.RdpConfig.Paste, !cfg.VncConfig.Paste)),
						"resize-method":         "display-update",
						// Set file transfer related parameters from config
						// DRIVE_ENABLE:           cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), cfg.RdpConfig.EnableDrive, false)),
						// DRIVE_PATH:             cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), cfg.RdpConfig.DrivePath, "")),
						// DRIVE_CREATE_PATH:      cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), cfg.RdpConfig.CreateDrivePath, false)),
						// DRIVE_DISABLE_UPLOAD:   cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), cfg.RdpConfig.DisableUpload, false)),
						// DRIVE_DISABLE_DOWNLOAD: cast.ToString(lo.Ternary(strings.Contains(protocol, "rdp"), cfg.RdpConfig.DisableDownload, false)),
						DRIVE_ENABLE:           "true",
						DRIVE_PATH:             fmt.Sprintf("/rdp/asset_%d", asset.Id),
						DRIVE_CREATE_PATH:      "true",
						DRIVE_DISABLE_UPLOAD:   "false",
						DRIVE_DISABLE_DOWNLOAD: "false",
						DRIVE_NAME:             "Drive",
					}
				}, func() map[string]string {
					return map[string]string{
						"width":     cast.ToString(w),
						"height":    cast.ToString(h),
						"dpi":       cast.ToString(dpi),
						"read-only": "true",
					}
				}),
		},
	}
	if t.ConnectionId == "" {
		t.SessionId = sessionId
		t.Config.Parameters["recording-name"] = t.SessionId
	}
	if gateway != nil && gateway.Id != 0 && t.ConnectionId == "" {
		t.gw, err = tunneling.OpenTunnel(false, t.SessionId, asset.Ip, cast.ToInt(port), gateway)
		if err != nil {
			return t, err
		}
		t.Config.Parameters["hostname"] = "localhost"
		t.Config.Parameters["port"] = cast.ToString(t.gw.LocalPort)
	}

	err = t.handshake()

	return
}

// handshake
//
//	https://guacamole.apache.org/doc/gug/guacamole-protocol.html#handshake-phase
func (t *Tunnel) handshake() (err error) {
	defer func() {
		if err != nil {
			t.conn.Close()
		}
	}()

	// select
	if _, err = t.WriteInstruction(NewInstruction("select", lo.Ternary(t.ConnectionId == "", t.Config.Protocol, t.ConnectionId))); err != nil {
		return
	}

	// args
	args, err := t.assert("args")
	if err != nil {
		return
	}
	parameters := make([]string, len(args.Args))
	for i, name := range args.Args {
		parameters[i] = t.Config.Parameters[name]
	}

	// size audio video image
	if _, err = t.WriteInstruction(NewInstruction("size", t.Config.Parameters["width"], t.Config.Parameters["height"], t.Config.Parameters["dpi"])); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("audio", "audio/L8")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("video")); err != nil {
		return
	}
	if _, err = t.WriteInstruction(NewInstruction("image", "image/jpeg", "image/png", "image/webp")); err != nil {
		return
	}

	// connect
	if _, err = t.WriteInstruction(NewInstruction("connect", parameters...)); err != nil {
		return
	}

	// ready
	ready, err := t.assert("ready")
	if err != nil {
		return
	}

	if len(ready.Args) == 0 {
		err = fmt.Errorf("empty connection id")
		return
	}

	t.ConnectionId = ready.Args[0]

	return
}

func (t *Tunnel) Write(p []byte) (n int, err error) {
	if t == nil || t.writer == nil {
		return
	}
	n, err = t.writer.Write(p)
	if err != nil {
		return
	}
	err = t.writer.Flush()
	return
}

func (t *Tunnel) WriteInstruction(instruction *Instruction) (n int, err error) {
	n, err = t.Write(instruction.Bytes())
	return
}

func (t *Tunnel) Read() (p []byte, err error) {
	p, err = t.reader.ReadBytes(delimiter)
	return
}

func (t *Tunnel) ReadInstruction() (*Instruction, error) {
	data, err := t.Read()
	if err != nil {
		return nil, err
	}

	instruction := (&Instruction{}).Parse(string(data))

	// Check if this is a file transfer instruction
	if isFileInstruction(instruction.Opcode) {
		return t.HandleFileInstruction(instruction)
	}

	return instruction, nil
}

// isFileInstruction checks if the instruction is related to file transfer
func isFileInstruction(opcode string) bool {
	return opcode == INSTRUCTION_FILE_UPLOAD ||
		opcode == INSTRUCTION_FILE_DOWNLOAD ||
		opcode == INSTRUCTION_FILE_DATA ||
		opcode == INSTRUCTION_FILE_COMPLETE
}

func (t *Tunnel) assert(opcode string) (instruction *Instruction, err error) {
	instruction, err = t.ReadInstruction()
	if err != nil {
		return
	}

	if opcode != instruction.Opcode {
		err = fmt.Errorf(`expect instruction "%s" but got "%s"`, opcode, instruction.Opcode)
	}

	return
}

func (t *Tunnel) Close() {
	tunneling.CloseTunnels(t.SessionId)
}

func (t *Tunnel) Disconnect() {
	logger.L().Debug("client disconnect")
	t.WriteInstruction(NewInstruction("disconnect"))
}

// HandleFileUpload handles file upload request
func (t *Tunnel) HandleFileUpload(filename string, size int64) (string, error) {
	if t.drivePath == "" || t.Config.Parameters[DRIVE_DISABLE_UPLOAD] == "true" {
		return "", fmt.Errorf("file upload is disabled")
	}

	transfer, err := t.transferManager.CreateUpload(t.SessionId, filename, t.drivePath)
	if err != nil {
		return "", err
	}

	return transfer.ID, nil
}

// HandleFileDownload handles file download request
func (t *Tunnel) HandleFileDownload(filename string) (string, int64, error) {
	if t.drivePath == "" || t.Config.Parameters[DRIVE_DISABLE_DOWNLOAD] == "true" {
		return "", 0, fmt.Errorf("file download is disabled")
	}

	transfer, err := t.transferManager.CreateDownload(t.SessionId, filename, t.drivePath)
	if err != nil {
		return "", 0, err
	}

	return transfer.ID, transfer.Size, nil
}

// WriteFileData writes data to an upload file
func (t *Tunnel) WriteFileData(transferId string, data []byte) (int, error) {
	transfer := t.transferManager.GetTransfer(transferId)
	if transfer == nil {
		return 0, fmt.Errorf("transfer not found: %s", transferId)
	}

	return transfer.Write(data)
}

// ReadFileData reads data from a download file
func (t *Tunnel) ReadFileData(transferId string, buffer []byte) (int, error) {
	transfer := t.transferManager.GetTransfer(transferId)
	if transfer == nil {
		return 0, fmt.Errorf("transfer not found: %s", transferId)
	}

	return transfer.Read(buffer)
}

// CloseFileTransfer closes a file transfer
func (t *Tunnel) CloseFileTransfer(transferId string) error {
	transfer := t.transferManager.GetTransfer(transferId)
	if transfer == nil {
		return fmt.Errorf("transfer not found: %s", transferId)
	}

	err := transfer.Close()
	t.transferManager.RemoveTransfer(transferId)
	return err
}

// SendDownloadData reads data from a file and sends to client
func (t *Tunnel) SendDownloadData(transferId string) error {
	transfer := t.transferManager.GetTransfer(transferId)
	if transfer == nil {
		return fmt.Errorf("transfer not found: %s", transferId)
	}

	if transfer.IsUpload {
		return fmt.Errorf("cannot download from upload transfer")
	}

	// Use 4KB buffer for file data
	buffer := make([]byte, 4096)

	for !transfer.Completed {
		n, err := transfer.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}

		if n > 0 {
			// Send file data to client
			dataInstr := NewInstruction(INSTRUCTION_FILE_DATA, transferId, string(buffer[:n]))
			if _, err := t.WriteInstruction(dataInstr); err != nil {
				return err
			}

			// Read ACK from client
			ack, err := t.ReadInstruction()
			if err != nil {
				return err
			}

			if ack.Opcode != INSTRUCTION_FILE_ACK {
				return fmt.Errorf("expected ACK instruction, got: %s", ack.Opcode)
			}
		}

		if err == io.EOF || transfer.Completed {
			break
		}
	}

	// Send complete instruction
	completeInstr := NewInstruction(INSTRUCTION_FILE_COMPLETE, transferId)
	if _, err := t.WriteInstruction(completeInstr); err != nil {
		return err
	}

	return t.CloseFileTransfer(transferId)
}
