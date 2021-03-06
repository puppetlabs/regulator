package localfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/puppetlabs/regulator/rgerror"
	"github.com/puppetlabs/regulator/validator"
)

const STDIN_IDENTIFIER string = "__STDIN__"

func readFromStdin() string {
	var builder strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		builder.WriteString(scanner.Text() + "\n")
	}
	return builder.String()
}

func ChooseFileOrStdin(specfile string, use_stdin bool) (string, *rgerror.RGerror) {
	if use_stdin {
		if len(specfile) > 0 {
			return "", &rgerror.RGerror{
				Kind:    rgerror.InvalidInput,
				Message: "Cannot specify both a file and to use stdin",
				Origin:  nil,
			}
		}
		return STDIN_IDENTIFIER, nil
	} else {
		// Validate that the thing is actually a file on disk before
		// going any further
		//
		// Cheat a little with the validator: this function is mostly used
		// for the CLI commands, so use a name that shows it's the flag
		rgerr := validator.ValidateParams(fmt.Sprintf(
			`[{"name":"--file","value":"%s","validate":["NotEmpty","IsFile"]}]`,
			specfile,
		))
		if rgerr != nil {
			return "", rgerr
		}
		return specfile, nil
	}
}

func ReadFileOrStdin(maybe_file string) ([]byte, *rgerror.RGerror) {
	var raw_data []byte
	var rgerr *rgerror.RGerror
	if maybe_file == STDIN_IDENTIFIER {
		raw_data = []byte(readFromStdin())
	} else {
		raw_data, rgerr = ReadFileInChunks(maybe_file)
		if rgerr != nil {
			return nil, rgerr
		}
	}
	return raw_data, nil
}

func ReadFileInChunks(location string) ([]byte, *rgerror.RGerror) {
	f, err := os.OpenFile(location, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to open file:\n%s", err),
			Origin:  err,
		}
	}
	defer f.Close()

	// Create a buffer, read 32 bytes at a time
	byte_buffer := make([]byte, 32)
	file_contents := make([]byte, 0)
	for {
		bytes_read, err := f.Read(byte_buffer)
		if bytes_read > 0 {
			file_contents = append(file_contents, byte_buffer[:bytes_read]...)
		}
		if err != nil {
			if err != io.EOF {
				return nil, &rgerror.RGerror{
					Kind:    rgerror.ExecError,
					Message: fmt.Sprintf("Failed to read file:\n%s", err),
					Origin:  err,
				}
			} else {
				break
			}
		}
	}
	return file_contents, nil
}

func OverwriteFile(location string, data []byte) *rgerror.RGerror {
	f, err := os.OpenFile(location, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to open file:\n%s", err),
			Origin:  err,
		}
	}
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to write to file:\n%s", err),
			Origin:  err,
		}
	}
	return nil
}
