package coreutils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-shellwords"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// Removes the provided flag and value from the command arguments
func RemoveFlagFromCommand(args *[]string, flagIndex, flagValueIndex int) {
	argsCopy := *args
	// Remove flag from Command if required.
	if flagIndex != -1 {
		argsCopy = append(argsCopy[:flagIndex], argsCopy[flagValueIndex+1:]...)
	}
	*args = argsCopy
}

// Find value of required CLI flag in Command.
// If flag does not exist, the returned index is -1 and nil is returned as the error.
// Return values:
// err - error if flag exists but failed to extract its value.
// flagIndex - index of flagName in Command.
// flagValueIndex - index in Command in which the value of the flag exists.
// flagValue - value of flagName.
func FindFlag(flagName string, args []string) (flagIndex, flagValueIndex int, flagValue string, err error) {
	flagIndex = -1
	flagValueIndex = -1
	for index, arg := range args {
		// Check current argument.
		if !strings.HasPrefix(arg, flagName) {
			continue
		}

		// Get flag value.
		flagValue, flagValueIndex, err = getFlagValueAndValueIndex(flagName, args, index)
		if err != nil {
			return
		}

		// If was not the correct flag, continue looking.
		if flagValueIndex == -1 {
			continue
		}

		// Return value.
		flagIndex = index
		return
	}

	// Flag not found.
	return
}

// Get the provided flag's value, and the index of the value.
// Value-index can either be same as flag's index, or the next one.
// Return error if flag is found, but couldn't extract value.
// If the provided index doesn't contain the searched flag, return flagIndex = -1.
func getFlagValueAndValueIndex(flagName string, args []string, flagIndex int) (flagValue string, flagValueIndex int, err error) {
	indexValue := args[flagIndex]

	// Check if flag is in form '--key=value'
	indexValue = strings.TrimPrefix(indexValue, flagName)
	if strings.HasPrefix(indexValue, "=") {
		if len(indexValue) > 1 {
			return indexValue[1:], flagIndex, nil
		}
		return "", -1, errorutils.CheckError(errors.New(fmt.Sprintf("Flag %s is provided with empty value.", flagName)))
	}

	// Check if it is a different flag with same prefix, e.g --server-id-another
	if len(indexValue) > 0 {
		return "", -1, nil
	}

	// If reached here, expect the flag value in next argument.
	if len(args) < flagIndex+2 {
		// Flag value does not exist.
		return "", -1, errorutils.CheckError(errors.New(fmt.Sprintf("Failed extracting value of provided flag: %s.", flagName)))
	}

	nextIndexValue := args[flagIndex+1]
	// Don't allow next value to be a flag.
	if strings.HasPrefix(nextIndexValue, "-") {
		// Flag value does not exist.
		return "", -1, errorutils.CheckError(errors.New(fmt.Sprintf("Failed extracting value of provided flag: %s.", flagName)))
	}

	return nextIndexValue, flagIndex + 1, nil
}

// Boolean flag can be provided in one of the following forms:
// 1. --flag=value, where value can be true/false
// 2. --flag, here the value is true
// Return values:
// flagIndex - index of flagName in args.
// flagValue - value of flagName.
// err - error if flag exists, but we failed to extract its value.
// If flag does not exist flagIndex = -1 with false value and nil error.
func FindBooleanFlag(flagName string, args []string) (flagIndex int, flagValue bool, err error) {
	var arg string
	for flagIndex, arg = range args {
		if strings.HasPrefix(arg, flagName) {
			value := strings.TrimPrefix(arg, flagName)
			if len(value) == 0 {
				flagValue = true
			} else if strings.HasPrefix(value, "=") {
				flagValue, err = strconv.ParseBool(value[1:])
			} else {
				continue
			}
			return
		}
	}
	return -1, false, nil
}

// Find the first match of any of the provided flags in args.
// Return same values as FindFlag.
func FindFlagFirstMatch(flags, args []string) (flagIndex, flagValueIndex int, flagValue string, err error) {
	// Look for provided flags.
	for _, flag := range flags {
		flagIndex, flagValueIndex, flagValue, err = FindFlag(flag, args)
		if err != nil {
			return
		}
		if flagIndex != -1 {
			// Found value for flag.
			return
		}
	}
	return
}

func ExtractInsecureTlsFromArgs(args []string) (cleanArgs []string, insecureTls bool, err error) {
	cleanArgs = append([]string(nil), args...)

	flagIndex, insecureTls, err := FindBooleanFlag("--insecure-tls", args)
	if err != nil {
		return
	}
	RemoveFlagFromCommand(&cleanArgs, flagIndex, flagIndex)
	return
}

func ExtractDetailedSummaryFromArgs(args []string) (cleanArgs []string, detailedSummary bool, err error) {
	cleanArgs = append([]string(nil), args...)

	flagIndex, detailedSummary, err := FindBooleanFlag("--detailed-summary", args)
	if err != nil {
		return
	}
	RemoveFlagFromCommand(&cleanArgs, flagIndex, flagIndex)
	return
}

func ExtractXrayScanFromArgs(args []string) (cleanArgs []string, xrayScan bool, err error) {
	cleanArgs = append([]string(nil), args...)

	flagIndex, xrayScan, err := FindBooleanFlag("--scan", args)
	if err != nil {
		return
	}
	RemoveFlagFromCommand(&cleanArgs, flagIndex, flagIndex)
	return
}

func ExtractXrayOutputFormatFromArgs(args []string) (cleanArgs []string, format string, err error) {
	cleanArgs = append([]string(nil), args...)

	flagIndex, valIndex, format, err := FindFlag("--format", args)
	if err != nil {
		return
	}
	RemoveFlagFromCommand(&cleanArgs, flagIndex, valIndex)
	return
}

// Iterate over each argument, if env variable is found (e.g $HOME) replace it with env value.
func ParseArgs(args []string) ([]string, error) {
	// Escape backslash & space
	for i := 0; i < len(args); i++ {
		args[i] = strings.ReplaceAll(args[i], `\`, `\\`)
		if strings.Index(args[i], ` `) != -1 && !isQuote(args[i]) {
			args[i] = strings.ReplaceAll(args[i], `"`, ``)
			args[i] = strings.ReplaceAll(args[i], `'`, ``)
			args[i] = `"` + args[i] + `"`
		}
	}
	parser := shellwords.NewParser()
	parser.ParseEnv = true
	return parser.Parse(strings.Join(args, " "))
}

func isQuote(s string) bool {
	return len(s) > 0 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\''))
}
