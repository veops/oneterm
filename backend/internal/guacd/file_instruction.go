package guacd

import (
	"fmt"
	"strconv"
)

// File transfer instruction constants
const (
	INSTRUCTION_FILE_UPLOAD   = "file-upload"
	INSTRUCTION_FILE_DOWNLOAD = "file-download"
	INSTRUCTION_FILE_DATA     = "file-data"
	INSTRUCTION_FILE_ACK      = "file-ack"
	INSTRUCTION_FILE_COMPLETE = "file-complete"
	INSTRUCTION_FILE_ERROR    = "file-error"
)

// Object instruction constants for filesystem operations
const (
	INSTRUCTION_FILESYSTEM = "filesystem"
	INSTRUCTION_GET        = "get"
	INSTRUCTION_PUT        = "put"
	INSTRUCTION_BODY       = "body"
	INSTRUCTION_UNDEFINE   = "undefine"
)

// Stream instruction constants
const (
	INSTRUCTION_BLOB = "blob"
	INSTRUCTION_END  = "end"
)

// Filesystem mimetypes
const (
	MIMETYPE_STREAM_INDEX = "application/vnd.glyptodon.guacamole.stream-index+json"
	MIMETYPE_TEXT_PLAIN   = "text/plain"
)

// HandleFileInstruction processes file transfer related instructions
func (t *Tunnel) HandleFileInstruction(instruction *Instruction) (*Instruction, error) {
	switch instruction.Opcode {
	case INSTRUCTION_FILE_UPLOAD:
		if len(instruction.Args) < 2 {
			return NewInstruction(INSTRUCTION_FILE_ERROR, "Invalid upload request"), nil
		}

		filename := instruction.Args[0]
		size, err := strconv.ParseInt(instruction.Args[1], 10, 64)
		if err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, "Invalid file size"), nil
		}

		transferId, err := t.HandleFileUpload(filename, size)
		if err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, err.Error()), nil
		}

		return NewInstruction(INSTRUCTION_FILE_ACK, transferId), nil

	case INSTRUCTION_FILE_DOWNLOAD:
		if len(instruction.Args) < 1 {
			return NewInstruction(INSTRUCTION_FILE_ERROR, "Invalid download request"), nil
		}

		filename := instruction.Args[0]
		transferId, size, err := t.HandleFileDownload(filename)
		if err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, err.Error()), nil
		}

		// Send acknowledgement with transfer ID and file size
		ackInstr := NewInstruction(INSTRUCTION_FILE_ACK, transferId, strconv.FormatInt(size, 10))
		if _, err := t.WriteInstruction(ackInstr); err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, fmt.Sprintf("Failed to send ACK: %s", err.Error())), nil
		}

		// Start file download process in a new goroutine
		go func() {
			if err := t.SendDownloadData(transferId); err != nil {
				// Log error, but we can't send error instruction here as it would interfere with protocol
				fmt.Printf("Download failed: %s\n", err.Error())
			}
		}()

		// Return nil to avoid sending another response
		return nil, nil

	case INSTRUCTION_FILE_DATA:
		if len(instruction.Args) < 2 {
			return NewInstruction(INSTRUCTION_FILE_ERROR, "Invalid data request"), nil
		}

		transferId := instruction.Args[0]
		data := []byte(instruction.Args[1])

		// If this is an upload, write the data
		n, err := t.WriteFileData(transferId, data)
		if err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, fmt.Sprintf("Write error: %s", err.Error())), nil
		}

		return NewInstruction(INSTRUCTION_FILE_ACK, transferId, strconv.Itoa(n)), nil

	case INSTRUCTION_FILE_COMPLETE:
		if len(instruction.Args) < 1 {
			return NewInstruction(INSTRUCTION_FILE_ERROR, "Invalid complete request"), nil
		}

		transferId := instruction.Args[0]

		err := t.CloseFileTransfer(transferId)
		if err != nil {
			return NewInstruction(INSTRUCTION_FILE_ERROR, fmt.Sprintf("Failed to complete transfer: %s", err.Error())), nil
		}

		return NewInstruction(INSTRUCTION_FILE_ACK, transferId, "complete"), nil

	default:
		return nil, fmt.Errorf("Unknown file instruction: %s", instruction.Opcode)
	}
}
