package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/puppetlabs/regulator/rgerror"
	"github.com/puppetlabs/regulator/version"
)

type Command struct {
	Verb        string
	Noun        string
	ExecutionFn func()
}

// shouldHaveArgs does two things:
// * validate that the number of args that aren't flags have been provided (i.e. the number of strings
//    after the command name that aren't flags)
// * parse the remaining flags
//
// If the wrong number of args is passed it prints helpful usage
func ShouldHaveArgs(num_args int, usage string, description string, flagset *flag.FlagSet) {
	real_args := num_args + 1
	passed_fs := flagset != nil
	for index, arg := range os.Args {
		if arg == "-h" {
			fmt.Fprintf(os.Stderr, "Usage:\n  %s\n\nDescription:\n  %s\n\n", usage, description)
			if passed_fs {
				fmt.Fprintf(os.Stderr, "Available flags:\n")
				flagset.PrintDefaults()
			}
			os.Exit(0)
		}
		// None of the arguments required by the command should start with dashes, if they
		// do assume an arg is missing and this is a flag
		if index <= num_args && strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "Error running command:\n\nInvalid input, not enough arguments.\n\nUsage:\n  %s\n\nDescription:\n  %s\n\n", usage, description)
			if passed_fs {
				fmt.Fprintf(os.Stderr, "Available flags:\n")
				flagset.PrintDefaults()
			}
			os.Exit(1)
		}
	}
	if len(os.Args) < real_args {
		fmt.Fprintf(os.Stderr, "Error running command:\n\nInvalid input, not enough arguments.\n\nUsage:\n  %s\n\nDescription:\n  %s\n\n", usage, description)
		if passed_fs {
			fmt.Fprintf(os.Stderr, "Available flags:\n")
			flagset.PrintDefaults()
		}
		os.Exit(1)
	} else if len(os.Args) > real_args && passed_fs {
		flagset.Parse(os.Args[real_args:])
	}
}

// handleCommandRGerror catches InvalidInput rgerror.RGerrors and prints usage
// if that was the error thrown. IF a different type of rgerror.RGerror is thrown
// it just prints the error.
//
// If the command succeeds handleCommandRGerror exits the whole go process
// with code 0
func HandleCommandRGerror(rgerr *rgerror.RGerror, usage string, description string, flagset *flag.FlagSet) {
	if rgerr != nil {
		if rgerr.Kind == rgerror.InvalidInput {
			fmt.Fprintf(os.Stderr, "%s\nUsage:\n  %s\n\nDescription:\n  %s\n\n", rgerr, usage, description)
			if flagset != nil {
				flagset.PrintDefaults()
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error running command:\n\n%s\n", rgerr)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func RunCommand(tool_name string, command_list []Command) {
	if len(os.Args) > 2 {
		for _, command := range command_list {
			if os.Args[1] == command.Verb && os.Args[2] == command.Noun {
				command.ExecutionFn()
			}
		}
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Fprintf(os.Stdout, "%s\n", version.VERSION)
			os.Exit(0)
		case "-h":
			// do nothing, it will print the usage message below
		default:
			// If we've arrived here, that means the args passed don't match an existing command
			// --version or -h
			fmt.Printf("Unknown %s command \"%s\"\n\n", tool_name, strings.Join(os.Args, " "))
		}
	}

	fmt.Printf("Usage:\n  %s [COMMAND] [OBJECT] [ARGUMENTS] [FLAGS]\n\nAvailable commands:\n", tool_name)
	for _, command := range command_list {
		fmt.Printf("    %s %s\n", command.Verb, command.Noun)
	}
	os.Exit(1)
}
