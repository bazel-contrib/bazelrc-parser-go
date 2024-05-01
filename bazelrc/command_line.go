package bazelrc

import (
	"fmt"
)

// CommandLineArgsAfterCommand represents the constituent components of the parsed arguments to an invocation of Bazel.
type CommandLineArgsAfterCommand struct {
	// Targets contains any targets (i.e. things that don't look like Bazel flags or flag-values) found
	Targets []string
	// BazelFlags contains anything we could recognize as a Bazel flag.
	BazelFlags BazelFlagValues
	// ExecutableArgs contains anything that came after a standalone `--` flag.
	ExecutableArgs []string
}

// ParseCommandLineArgsAfterCommand parses a command line (rather than a bazelrc line) to find a list of targets and arguments there-to.
// This function expects to be given a slice which comes after the command (i.e. for `bazel --host_jvm_debug build //blah --jobs=10` it should be passed `["//blah", "--jobs=10"]`.
// This function lives here to re-use most of the internals of bazelrc parsing, but is unrelated to bazelrc files itself.
func ParseCommandLineArgsAfterCommand(knownFlagData *FlagData, tokens []string) (*CommandLineArgsAfterCommand, error) {
	// We don't have a workspace directory, so we don't have a value to pass here.
	// Fortunately, our command line also can't include `import` directives,
	// which is the only thing the workspace directory is used for, so this doesn't really matter.
	parser := NewBazelRcParser("", knownFlagData)
	argAccumulator := make(map[string][]string)
	var targetsAndArgsAccumulator []string
	var flagNameExpectingValueWithLeadingDashes *string
	continuation, err := parser.parseTokenizedArgsOnSingleLineAfterCommand(tokens, argAccumulator, &targetsAndArgsAccumulator, &flagNameExpectingValueWithLeadingDashes, nil, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bazel command line: %w", err)
	}
	if continuation {
		return nil, fmt.Errorf("didn't understand continuation \\ at end of command")
	}
	if flagNameExpectingValueWithLeadingDashes != nil {
		return nil, fmt.Errorf("internal error: ended parsing Bazel command line while expecting value for flag %q", *flagNameExpectingValueWithLeadingDashes)
	}

	targets := targetsAndArgsAccumulator
	var args []string
	for i, v := range targetsAndArgsAccumulator {
		if v == "--" {
			targets = targetsAndArgsAccumulator[:i]
			args = targetsAndArgsAccumulator[i+1:]
			break
		}
	}

	ret := &CommandLineArgsAfterCommand{
		Targets:        targets,
		BazelFlags:     argAccumulator,
		ExecutableArgs: args,
	}

	return ret, nil
}
