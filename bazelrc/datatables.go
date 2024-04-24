package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"

	"google.golang.org/protobuf/proto"

	"github.com/bazel-contrib/bazelrc-parser-go/bazel_protos/bazel_flags"
)

// FlagData holds information about Bazel's flag schema (i.e. metadata about the flag definitions).
// Note that this schema changes across different Bazel versions.
type FlagData struct {
	// BooleanFlags contains the flags which are enabled without specifying a value for the flag (e.g. --subcommands).
	BooleanFlags map[string]bool
	// FlagAbbreviation maps short names to long names, e.g. maps `j` to `jobs`.
	FlagAbbreviations map[string]string
}

// GetFlagDataFromBazel returns a FlagData by invoking bazel to learn about flags.
// The passed command should be an exec.Cmd which is configured to run bazel in the correct directory with whatever startup args are needed.
// This function will add a bazel commands and flags to that command, and redirect the stdout.
// It relies on features only present from Bazel 7, and will return misleading data for BooleanFlags if run with an older Bazel.
func GetFlagDataFromBazel(command *exec.Cmd) (*FlagData, error) {
	var protoByteBuffer bytes.Buffer
	command.Args = append(command.Args, "help", "flags-as-proto")
	command.Stdout = &protoByteBuffer
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("failed to run bazel help flags-as-proto: %w", err)
	}
	protoBytes, err := base64.StdEncoding.DecodeString(protoByteBuffer.String())
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode Bazel's known flags proto file: %w", err)
	}

	var flags bazel_flags.FlagCollection

	if err := proto.Unmarshal(protoBytes, &flags); err != nil {
		return nil, fmt.Errorf("failed to unmarshall Bazel proto for flags: %w", err)
	}

	booleanFlags := make(map[string]bool)
	flagAbbreviations := make(map[string]string)

	for _, flag := range flags.FlagInfos {
		booleanFlags[flag.GetName()] = !flag.GetRequiresValue()
		if flag.Abbreviation != nil {
			if len(flag.GetAbbreviation()) != 1 {
				return nil, fmt.Errorf("saw flag %q abbreviates to %q but expect all flag abbreviations to be single characters", flag.GetName(), flag.GetAbbreviation())
			}
			flagAbbreviations[flag.GetAbbreviation()] = flag.GetName()
		}
	}

	return &FlagData{
		BooleanFlags:      booleanFlags,
		FlagAbbreviations: flagAbbreviations,
	}, nil
}
