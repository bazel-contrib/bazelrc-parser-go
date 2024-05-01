package parser

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/shlex"
	"golang.org/x/exp/slices"
)

// NewBazelRcParser makes a parser of bazelrc files.
func NewBazelRcParser(workspaceDirectory string, knownFlagData *FlagData) *BazelRcParser {
	return &BazelRcParser{
		workspaceDirectory: workspaceDirectory,
		knownFlagData:      knownFlagData,
	}
}

type BazelRcParser struct {
	workspaceDirectory string
	knownFlagData      *FlagData
}

// Parsefile parses a bazelrc file.
func (p *BazelRcParser) Parsefile(file io.Reader, filePath string) (*BazelrcContents, error) {
	contents := newBazelrcContents()
	return contents, p.parseFileInternal(contents, file, []string{filePath})
}

// importCallStack should always contain at least one element. This func is called by Parsefile which passes top level path, any recursive calls append to the importCallStack slice.
func (p *BazelRcParser) parseFileInternal(out *BazelrcContents, file io.Reader, importCallStack []string) error {
	byteValue, err := io.ReadAll(file)
	if err != nil {
		return makeError(importCallStack, -1, fmt.Errorf("failed to read file: %w", err), false)
	}

	var flagNameExpectingValueWithLeadingDashes *string

	lines := strings.Split(string(byteValue), "\n")
	for zeroBaseLineNumber := 0; zeroBaseLineNumber < len(lines); zeroBaseLineNumber++ {
		line := lines[zeroBaseLineNumber]

		// Strip out comments
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		tokens, err := tokenizeLine(line, importCallStack, zeroBaseLineNumber)
		if err != nil {
			return err
		}
		if len(tokens) == 0 {
			continue
		}

		commandName := tokens[0]
		commandArgumentsCount := len(tokens[1:])

		if commandName == "import" || commandName == "try-import" {
			if commandArgumentsCount != 1 {
				return makeError(importCallStack, zeroBaseLineNumber, fmt.Errorf("expected exactly 1 argument after %v, but got %v", commandName, commandArgumentsCount), false)
			}

			pathFinal := strings.ReplaceAll(tokens[1], "%workspace%", p.workspaceDirectory)
			importCallStackCopy := make([]string, len(importCallStack))
			copy(importCallStackCopy, importCallStack)

			importCycleErrorMessage := "bazelrc file contains import cycle"

			if slices.Contains(importCallStackCopy, pathFinal) {
				importCallStackCopy = append(importCallStackCopy, pathFinal)
				return makeError(importCallStackCopy, zeroBaseLineNumber, fmt.Errorf(importCycleErrorMessage), false)
			} else {
				importCallStackCopy = append(importCallStackCopy, pathFinal)
			}

			file, err := os.Open(pathFinal)
			if err != nil {
				if commandName == "import" {
					return makeError(importCallStackCopy, zeroBaseLineNumber, fmt.Errorf("unable to open file: %w", err), false)
				} else {
					continue
				}
			}
			defer file.Close()

			err = p.parseFileInternal(out, file, importCallStackCopy)
			if err != nil {
				// Avoid repeating the same error prefix repeatedly per file in the import cycle
				if strings.Contains(err.Error(), importCycleErrorMessage) {
					return err
				}
				return makeError(importCallStackCopy, zeroBaseLineNumber, fmt.Errorf("unable to parse import file due to error: %w", err), true)
			}
			continue
		}

		if len(tokens) < 2 {
			// This means the user possibly input: Just a command, or just a space or no space between command and flag.
			// In all of these cases, there is nothing useful for us to do.
			continue
		}

		_, alreadyContainsMap := out.entries[commandName]
		if !alreadyContainsMap {
			out.entries[commandName] = make(map[string][]string)
		}

		// Accumulate and discard any targets found.
		// We may want to do something with them in the future, but for now, none of our use-cases need that.
		//
		// An example line here may be
		//   `run:repin --action_env=REPIN=true @maven//:repin`
		// which allows someone to run `bazel run --config=repin` rather than needing to know all the flags and stuff.
		//
		// It's not generally encouraged to use bazelrc files like this, but it is supported, so we should support it.
		var targets []string

		if err := p.parseLineWithoutCommandPrefix(tokens[1:], lines, &zeroBaseLineNumber, out.entries[commandName], &targets, &flagNameExpectingValueWithLeadingDashes, importCallStack); err != nil {
			return err
		}
	}

	return err
}

func tokenizeLine(line string, importCallStack []string, zeroBaseLineNumber int) ([]string, error) {
	tokens, err := shlex.Split(line)
	if err != nil {
		return nil, makeError(importCallStack, zeroBaseLineNumber, fmt.Errorf("unable to split line: %w", err), false)
	}
	return tokens, nil
}

func (p *BazelRcParser) parseLineWithoutCommandPrefix(tokens []string, lines []string, zeroBaseLineNumber *int, entriesForCommand map[string][]string, targetAccumulator *[]string, flagNameExpectingValueWithLeadingDashes **string, importCallStack []string) error {
	for i, token := range tokens {
		parseNextLineAsContinuation, parseRestOfLineAsTargets, err := p.parseToken(token, i+1 == len(tokens), entriesForCommand, targetAccumulator, flagNameExpectingValueWithLeadingDashes, importCallStack, *zeroBaseLineNumber)
		if err != nil {
			return err
		}
		if parseNextLineAsContinuation {
			*zeroBaseLineNumber += 1
			if *zeroBaseLineNumber == len(lines) {
				return nil
			}
			tokens, err := tokenizeLine(lines[*zeroBaseLineNumber], importCallStack, *zeroBaseLineNumber)
			if err != nil {
				return err
			}
			if err := p.parseLineWithoutCommandPrefix(tokens, lines, zeroBaseLineNumber, entriesForCommand, targetAccumulator, flagNameExpectingValueWithLeadingDashes, importCallStack); err != nil {
				return err
			}
		}
		if parseRestOfLineAsTargets {
			*targetAccumulator = append(*targetAccumulator, tokens[i+1:]...)
			return nil
		}
	}
	return nil
}

func (p *BazelRcParser) parseTokenizedArgsOnSingleLineAfterCommand(tokens []string, argAccumulator map[string][]string, targetAccumulator *[]string, flagNameExpectingValueWithLeadingDashes **string, importCallStack []string, zeroBaseLineNumber int) (bool, error) {
	for i, token := range tokens {
		parseNextLineAsContinuation, parseRestOfLineAsTargets, err := p.parseToken(token, i+1 == len(tokens), argAccumulator, targetAccumulator, flagNameExpectingValueWithLeadingDashes, importCallStack, zeroBaseLineNumber)
		if err != nil {
			return false, err
		}
		if parseNextLineAsContinuation {
			return true, nil
		}
		if parseRestOfLineAsTargets {
			*targetAccumulator = append(*targetAccumulator, tokens[i+1:]...)
			return false, nil
		}
	}
	return false, nil
}

// Return values:
// * bool: Whether this token means the next line should be parsed as a continuation of this one.
// * bool: Whether this token means that the rest of the line should be treated as targets and accumulated in targetAccumulator (which this function can't do, because it only sees one token at a time).
// * error: Whether a fatal error occurred while parsing.
// At most one of the two boolean return values will be true.
func (p *BazelRcParser) parseToken(token string, isLastTokenInLine bool, entriesForCommand map[string][]string, targetAccumulator *[]string, flagNameExpectingValueWithLeadingDashes **string, importCallStack []string, zeroBaseLineNumber int) (bool, bool, error) {
	if isLastTokenInLine && token == "\\" {
		return true, false, nil
	}

	if *flagNameExpectingValueWithLeadingDashes != nil {
		flagNameExpectingValueWithoutLeadingDashes := stripLeadingDashes(**flagNameExpectingValueWithLeadingDashes)
		if p.isKnownBooleanFlag(flagNameExpectingValueWithoutLeadingDashes) && token != "true" && token != "false" {
			if err := p.handleBooleanFlag(**flagNameExpectingValueWithLeadingDashes, entriesForCommand, importCallStack, zeroBaseLineNumber); err != nil {
				return false, false, err
			}
			*flagNameExpectingValueWithLeadingDashes = nil
		} else {
			entriesForCommand[flagNameExpectingValueWithoutLeadingDashes] = append(entriesForCommand[flagNameExpectingValueWithoutLeadingDashes], token)
			*flagNameExpectingValueWithLeadingDashes = nil
			return false, false, nil
		}
	}

	if !strings.HasPrefix(token, "-") {
		*targetAccumulator = append(*targetAccumulator, token)
		return false, false, nil
	}

	if token == "--" {
		*targetAccumulator = append(*targetAccumulator, token)
		return false, true, nil
	}

	// We know it starts with a -, but doesn't start with -- which means this is maybe an abbreviated flag.
	// Abbreviated flags don't support being set with =. The only allowed abbreviated flag forms are:
	// For boolean flags:
	//  * -s
	//  * -s-
	// For non-boolean flags:
	//  * -j 200
	// We handle all of these cases here, rather than in the main loop because special-casing
	// handling of -s-, as well as the fact that = isn't allowed would otherwise overly complicate
	// the non-abbreviated-flag codepath.
	if !strings.HasPrefix(token, "--") {
		fullFlagName, value, fullFlagNameWithLeadingDashes, err := p.parseAsAbbreviatedFlag(token)
		if err != nil {
			return false, false, err
		} else if fullFlagNameWithLeadingDashes != "" {
			*flagNameExpectingValueWithLeadingDashes = &fullFlagNameWithLeadingDashes
		} else {
			entriesForCommand[fullFlagName] = append(entriesForCommand[fullFlagName], value)
		}
		return false, false, nil
	}

	if strings.Contains(token, "=") {
		parts := strings.SplitN(token, "=", 2)
		flagName := parts[0]

		flagName = stripLeadingDashes(flagName)
		flagValue := parts[1]

		entriesForCommand[flagName] = append(entriesForCommand[flagName], flagValue)
	} else {
		if isLastTokenInLine {
			if err := p.handleBooleanFlag(token, entriesForCommand, importCallStack, zeroBaseLineNumber); err != nil {
				return false, false, err
			}
		} else {
			*flagNameExpectingValueWithLeadingDashes = &token
		}
	}
	return false, false, nil
}

func (p *BazelRcParser) isKnownBooleanFlag(flagName string) bool {
	knownBoolean, knownAtAll := p.knownFlagData.BooleanFlags[flagName]
	if knownAtAll {
		return knownBoolean
	}
	if !strings.HasPrefix(flagName, "no") {
		return false
	}
	return p.knownFlagData.BooleanFlags[flagName[2:]]
}

// Returns exactly one of:
//   - First two return values are a flag name and value.
//   - Third return value is a full flag name with leading dashes, which is expecting a value.
//   - Fourth return value is an error.
func (p *BazelRcParser) parseAsAbbreviatedFlag(token string) (string, string, string, error) {
	if len(token) < 2 {
		return "", "", "", fmt.Errorf("%q isn't a valid flag", token)
	}
	// We assume that all abbreviations are of length exactly 1.
	// This is verified in our data table generator.
	possibleAbbreviation := token[1:2]
	fullFlagName, isAbbreviation := p.knownFlagData.FlagAbbreviations[possibleAbbreviation]
	if !isAbbreviation {
		return "", "", "", fmt.Errorf("flag %s wasn't a known abbreviation", possibleAbbreviation)
	}
	fullFlagNameWithLeadingDashes := fmt.Sprintf("--%s", fullFlagName)
	if isBoolean := p.knownFlagData.BooleanFlags[fullFlagName]; isBoolean {
		if len(token) == 2 {
			return fullFlagName, "true", "", nil
		} else if len(token) == 3 && token[2:3] == "-" {
			return fullFlagName, "false", "", nil
		} else {
			return "", "", "", fmt.Errorf("token %q wasn't a valid abbreviated flag", token)
		}
	} else if len(token) == 2 {
		return "", "", fullFlagNameWithLeadingDashes, nil
	} else {
		return "", "", "", fmt.Errorf("flag %q wasn't a valid abbreviated flag", token)
	}
}

func stripLeadingDashes(flag string) string {
	if strings.HasPrefix(flag, "--") {
		return flag[2:]
	} else if strings.HasPrefix(flag, "-") {
		return flag[1:]
	}
	return flag
}

func (p *BazelRcParser) handleBooleanFlag(flag string, flagNamesToValues map[string][]string, importCallStack []string, oneBaseLineNumber int) error {
	assumedValue := "true"
	flagName := stripLeadingDashes(flag)
	if strings.HasPrefix(flag, "--no") {
		assumedValue = "false"
		flagName = flag[4:]
	}
	if requiresValue := !p.knownFlagData.BooleanFlags[flagName]; requiresValue {
		return makeError(importCallStack, oneBaseLineNumber, fmt.Errorf("value-requiring flag %s didn't have value", flagName), false)
	}
	flagNamesToValues[flagName] = append(flagNamesToValues[flagName], assumedValue)
	return nil
}

func makeError(importCallStack []string, zeroBasedLineNumber int, errorMessage error, handlingImport bool) error {
	importReason := ""
	if len(importCallStack)-1 > 1 {
		importReason = fmt.Sprintf(" - File %s imports file %s", importCallStack[0], strings.Join(importCallStack[1:], " which imports file "))
	}
	prefixMessage := ""
	if !handlingImport {
		lineDescription := ""
		if zeroBasedLineNumber >= 0 {
			lineDescription = fmt.Sprintf(" on line %d", zeroBasedLineNumber+1)
		}
		prefixMessage = fmt.Sprintf("failed to process %s%s, ", importCallStack[len(importCallStack)-1], lineDescription)
	}

	return fmt.Errorf("%s%w%s", prefixMessage, errorMessage, importReason)
}
