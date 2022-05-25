package connection

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/puppetlabs/regulator/rgerror"
	"github.com/puppetlabs/regulator/sanitize"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Based on https://pkg.go.dev/golang.org/x/crypto/ssh/agent#example-NewClient
func openConnectionWithAgent(username string, target string, port string) (*ssh.Client, *rgerror.RGerror) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to connect to ssh agent"),
			Origin:  err,
		}
	}
	agentClient := agent.NewClient(conn)
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			// Use a callback rather than PublicKeys so we only consult the
			// agent once the remote server wants it.
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ssh_client, err := ssh.Dial("tcp", target+":"+port, config)
	if err != nil {
		return nil, &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to open ssh connection to %s", target),
			Origin:  err,
		}
	}
	return ssh_client, nil
}

func RunSSHCommand(command string, send_stdin string, username string, target string, port string) (string, string, int, *rgerror.RGerror) {
	client, rgerr := openConnectionWithAgent(username, target, port)
	if rgerr != nil {
		return "", "", -1, rgerr
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to open new ssh session to %s", target),
			Origin:  err,
		}
	}
	defer session.Close()
	var read_stdout, read_stderr bytes.Buffer
	session.Stdout = &read_stdout
	session.Stderr = &read_stderr
	if len(send_stdin) > 0 {
		session.Stdin = strings.NewReader(send_stdin)
	}
	err = session.Run(command)
	command_stdout := sanitize.ReplaceAllNewlines(read_stdout.String())
	command_stderr := sanitize.ReplaceAllNewlines(read_stderr.String())
	if err != nil {
		// This whole thing is insane, but when session.Run() returns
		// from executing a remote command and the command returned
		// an exit code other than 0 the Run() call returns an error,
		// which is fine, except it's not always the same type of error
		// so you have to use a type assertion https://go.dev/tour/methods/15
		// to find if the error was of type ExitError which you can fetch
		// the exit code from.
		//
		// https://pkg.go.dev/golang.org/x/crypto/ssh#Session.
		//
		// All I ever wanted to do was return the exit code from this function
		if exitError, ok := err.(*ssh.ExitError); ok {
			exit_status := exitError.Waitmsg.ExitStatus()
			return command_stdout, command_stderr, exit_status, &rgerror.RGerror{
				Kind: rgerror.RemoteExecError,
				Message: fmt.Sprintf("Remote command \"%s\" exited with non-zero exit status %d\n\nStdout:\n%s\nStderr:\n%s\n",
					command,
					exit_status,
					command_stdout,
					command_stderr),
				Origin: err,
			}
		} else {
			return command_stdout, command_stderr, -1, &rgerror.RGerror{
				Kind: rgerror.RemoteExecError,
				Message: fmt.Sprintf("Remote command \"%s\" exited with non-zero exit status %s\n\nStdout:\n%s\nStderr:\n%s\n",
					command,
					"unknown",
					command_stdout,
					command_stderr),
				Origin: err,
			}
		}
	}
	return command_stdout, command_stderr, 0, nil
}
