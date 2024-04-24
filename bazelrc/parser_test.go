package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/stretchr/testify/require"
)

func TestBazelrcParser(t *testing.T) {
	goodImportFilePath1, err := runfiles.Rlocation("bazelrc_parser_go/bazelrc/testdata/importable.bazelrc")
	require.NoError(t, err)
	badImportFilePath2, err := runfiles.Rlocation("bazelrc_parser_go/bazelrc/testdata/invalid-importable.bazelrc")
	require.NoError(t, err)

	testDir, err := os.MkdirTemp("", "example")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	tempTestFile := newFile(t, testDir, "temp-bazelrc", fmt.Sprintf("import %s", goodImportFilePath1))
	require.NoError(t, err)

	newFile(t, testDir, "workspace-bazelrc", "test --workspace=true")
	require.NoError(t, err)

	cycleBazelrcFile1 := newFile(t, testDir, "cycle-bazelrc1", fmt.Sprintf("import %s/cycle-bazelrc2", testDir))
	cycleBazelrcFile2 := newFile(t, testDir, "cycle-bazelrc2", fmt.Sprintf("import %s/cycle-bazelrc3", testDir))
	cycleBazelrcFile3 := newFile(t, testDir, "cycle-bazelrc3", fmt.Sprintf("import %s/cycle-bazelrc1", testDir))

	type testCase struct {
		input                  string
		wantOutput             *BazelrcContents
		expectedErrorSubstring string
	}
	for name, tc := range map[string]testCase{
		"simple one line one flag": {
			input: "build --foo=bar",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar"},
					},
				},
			},
		},
		"Gives error on unmatched quote": {
			input:                  `build --foo="bar`,
			wantOutput:             nil,
			expectedErrorSubstring: "unable to split line: EOF found when expecting closing quote",
		},
		"Gives error on unmatched single quote": {
			input:                  `build --foo='bar`,
			wantOutput:             nil,
			expectedErrorSubstring: "unable to split line: EOF found when expecting closing quote",
		},
		"Ignores commented lines": {
			input: `#build --ignored=value
					build --foo=bar`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar"},
					},
				},
			},
		},
		"Ignores empty lines": {
			input: `" "
					build --foo=bar`,

			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar"},
					},
				},
			},
		},
		"Ignores whitespace lines": {
			// The first line is just a trailing whitespace.
			// It's constructed awkwardy in a string because otherwise gofmt will remove the trailing whitespace.
			input: " \n" + `build --foo=bar`,

			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar"},
					},
				},
			},
		},
		"Ignores lines with no flags": {
			input: `build
					build --foo=bar`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar"},
					},
				},
			},
		},
		"Values may contain equals signs": {
			input: "build --foo=bar=true",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"bar=true"},
					},
				},
			},
		},
		"Single-quoted values are treated as single values": {
			input: "test  --test_env=FLAKY='maybe, maybe not'",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"test": {
						"test_env": []string{"FLAKY=maybe, maybe not"},
					},
				},
			},
		},
		"Double-quoted values are treated as single values": {
			input: `test  --test_env=FLAKY="maybe, maybe not"`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"test": {
						"test_env": []string{"FLAKY=maybe, maybe not"},
					},
				},
			},
		},
		"Flag-value pair with no equals sign": {
			input: "build --foo true",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"true"},
					},
				},
			},
		},
		"Value with equals sign where flagname has no equals sign.": {
			input: "build --foo strict=false",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"strict=false"},
					},
				},
			},
		},
		"Flag/value pair with no equals followed by pair with equals.": {
			input: "build --foo strict=false --bar=true",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"strict=false"},
						"bar": []string{"true"},
					},
				},
			},
		},
		"no value set doesn't require value": {
			input: "build --bool_flag",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag": []string{"true"},
					},
				},
			},
		},
		"no value set requires value": {
			input:                  "build --jobs",
			expectedErrorSubstring: "value-requiring flag jobs didn't have value",
		},
		"no value set followed by single dash flag line": {
			input: `build --bool_flag -o`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag":       []string{"true"},
						"other_bool_flag": []string{"true"},
					},
				},
			},
		},
		"boolean and equals sign flag": {
			input: "build --bool_flag --foo=hello",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag": []string{"true"},
						"foo":       []string{"hello"},
					},
				},
			},
		},
		"boolean has '--no' prefix": {
			input: "build --nobool_flag",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag": []string{"false"},
					},
				},
			},
		},
		"boolean has '--no' prefix & flag after it": {
			input: "build --nobool_flag --other_bool_flag",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag":       []string{"false"},
						"other_bool_flag": []string{"true"},
					},
				},
			},
		},
		"Mix of boolean and non-boolean flags": {
			input: "build --bool_flag --string_flag=123 --nobool_flag --int_flag=789",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag":   []string{"true", "false"},
						"string_flag": []string{"123"},
						"int_flag":    []string{"789"},
					},
				},
			},
		},
		"No arguments after import": {
			input: `build --bool_flag
					import`,
			wantOutput:             nil,
			expectedErrorSubstring: "failed to process /sample/bazelrc on line 2, expected exactly 1 argument after import, but got 0",
		},
		"No arguments after try-import": {
			input: `build --bool_flag
					try-import`,
			wantOutput:             nil,
			expectedErrorSubstring: "failed to process /sample/bazelrc on line 2, expected exactly 1 argument after try-import, but got 0",
		},
		">1 arguments after import": {
			input:                  "import path/to/file path2/to/file",
			wantOutput:             nil,
			expectedErrorSubstring: "failed to process /sample/bazelrc on line 1, expected exactly 1 argument after import, but got 2",
		},
		">1 arguments after try-import": {
			input:                  "try-import path/to/file path2/to/file",
			wantOutput:             nil,
			expectedErrorSubstring: "failed to process /sample/bazelrc on line 1, expected exactly 1 argument after try-import, but got 2",
		},
		"missing file 'import'": {
			input:                  "import /pathdoesnotexist/to/file",
			wantOutput:             nil,
			expectedErrorSubstring: "failed to process /pathdoesnotexist/to/file on line 1, unable to open file: open /pathdoesnotexist/to/file: no such file or directory",
		},
		"missing file 'try-import'": {
			input: "try-import /pathdoesnotexistpath/to/file",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{},
			},
		},
		"success on 'import' command": {
			input: fmt.Sprintf("import %s", goodImportFilePath1),
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"true"},
					},
				},
			},
		},
		"success on 'import' and 'non import' commands": {
			input: fmt.Sprintf(`build --foo=false
import %s
build --bar="true"`, goodImportFilePath1),

			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"false", "true"},
						"bar": []string{"true"},
					},
				},
			},
		},
		"success on 'try-import' command": {
			input: fmt.Sprintf("try-import %s", goodImportFilePath1),
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"true"},
					},
				},
			},
		},
		"bad imported file on 'import' command": {
			input:                  fmt.Sprintf("import %s", badImportFilePath2),
			wantOutput:             nil,
			expectedErrorSubstring: fmt.Sprintf("unable to parse import file due to error: failed to process %s on line 1, unable to split line", badImportFilePath2),
		},
		"bad imported file on 'try-import' command": {
			input:                  fmt.Sprintf("try-import %s", badImportFilePath2),
			wantOutput:             nil,
			expectedErrorSubstring: fmt.Sprintf("unable to parse import file due to error: failed to process %s on line 1, unable to split line", badImportFilePath2),
		},
		"recursive import": {
			input: fmt.Sprintf("import %s", tempTestFile),
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"true"},
					},
				},
			},
		},
		"recursive try-import": {
			input: fmt.Sprintf("try-import %s", tempTestFile),
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"true"},
					},
				},
			},
		},
		"workspace import": {
			input: "import %workspace%/workspace-bazelrc",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"test": {
						"workspace": []string{"true"},
					},
				},
			},
		},
		"workspace try-import": {
			input: "try-import %workspace%/workspace-bazelrc",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"test": {
						"workspace": []string{"true"},
					},
				},
			},
		},
		"error on import cycle": {
			input:      fmt.Sprintf("import %s", cycleBazelrcFile1),
			wantOutput: nil,
			expectedErrorSubstring: fmt.Sprintf(
				"failed to process %v on line 1, bazelrc file contains import cycle - File /sample/bazelrc imports file %v which imports file %v which imports file %v which imports file %v",
				cycleBazelrcFile1,
				cycleBazelrcFile1,
				cycleBazelrcFile2,
				cycleBazelrcFile3,
				cycleBazelrcFile1,
			),
		},
		"multiple imports of the same file": {
			input: `import %workspace%/workspace-bazelrc
import %workspace%/workspace-bazelrc`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"test": {
						"workspace": []string{"true", "true"},
					},
				},
			},
		},
		"single dash flag followed by double dash flag": {
			input: `build -b --double=value`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag": []string{"true"},
						"double":    []string{"value"},
					},
				},
			},
		},
		"single dash flag with no equals sign": {
			input: `build -j 200`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"jobs": []string{"200"},
					},
				},
			},
		},
		"double dash flag with no equals sign followed by single dash flag": {
			input: `build --foo "hello" -b`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo":       []string{"hello"},
						"bool_flag": []string{"true"},
					},
				},
			},
		},
		"mixed abbreviated and non-abbreviated (boolean)": {
			input: `build --bool_flag -b- -b --nobool_flag --bool_flag=true --bool_flag false`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"bool_flag": []string{"true", "false", "true", "false", "true", "false"},
					},
				},
			},
		},
		"mixed abbreviated and non-abbreviated (non-boolean)": {
			input: `build --jobs=100 -j 200 --jobs 300 -j 400`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"jobs": []string{"100", "200", "300", "400"},
					},
				},
			},
		},
		"continuation line with following flag": {
			input: `build --foo "hello" \\
--bar="baz"`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
						"bar": []string{"baz"},
					},
				},
			},
		},
		"continuation line with value on new line": {
			input: `build --foo "hello" --bar \\
baz`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
						"bar": []string{"baz"},
					},
				},
			},
		},
		"continuation line without following flag": {
			input: `build --foo "hello" \\
`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
					},
				},
			},
		},
		"continuation line with following flag followed by another line": {
			input: `build --foo "hello" \\
--bar="baz"
test --something=else`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
						"bar": []string{"baz"},
					},
					"test": {
						"something": []string{"else"},
					},
				},
			},
		},
		"continuation line with value on new line followed by another line": {
			input: `build --foo "hello" --bar \\
baz
test --something=else`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
						"bar": []string{"baz"},
					},
					"test": {
						"something": []string{"else"},
					},
				},
			},
		},
		"continuation line without following line": {
			input: `build --foo "hello" \\`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build": {
						"foo": []string{"hello"},
					},
				},
			},
		},
		"unknown abbreviated flags are forbidden": {
			input:                  `build -z`,
			expectedErrorSubstring: "flag z wasn't a known abbreviation",
		},
		"unknown negated abbreviated flags are forbidden": {
			input:                  `build -z-`,
			expectedErrorSubstring: "flag z wasn't a known abbreviation",
		},
		"multi-char abbreviated flags are forbidden": {
			input:                  `build -zqe`,
			expectedErrorSubstring: "flag z wasn't a known abbreviation",
		},
		"abbreviated flags cannot be concatenated": {
			input:                  `build -bo`,
			expectedErrorSubstring: "token \"-bo\" wasn't a valid abbreviated flag",
		},
		"abbreviated flags aren't allowed equals values": {
			input:                  `build -j=200`,
			expectedErrorSubstring: "flag \"-j=200\" wasn't a valid abbreviated flag",
		},
		"abbreviated flags aren't allowed inline values": {
			input:                  `build -j200`,
			expectedErrorSubstring: "flag \"-j200\" wasn't a valid abbreviated flag",
		},
		"copt space flag": {
			input: "build:asan --copt -fsanitize=address",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build:asan": {
						"copt": []string{"-fsanitize=address"},
					},
				},
			},
		},
		"copt space short flag not abbreviated flag": {
			input: "build:asan --copt -g",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build:asan": {
						"copt": []string{"-g"},
					},
				},
			},
		},
		"copt space abbreviated flag": {
			input: "build:asan --copt -b",
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build:asan": {
						"copt": []string{"-b"},
					},
				},
			},
		},
		"copt space quoted multiple flags": {
			input: `build:asan --copt "--foo --bar"`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build:asan": {
						"copt": []string{"--foo --bar"},
					},
				},
			},
		},
		"copt space quoted abbreviated flag with value": {
			input: `build:asan --copt "-b c"`,
			wantOutput: &BazelrcContents{
				entries: map[string]BazelFlagValues{
					"build:asan": {
						"copt": []string{"-b c"},
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			flagData := &FlagData{
				BooleanFlags: map[string]bool{
					"bool_flag":       true,
					"other_bool_flag": true,
				},
				FlagAbbreviations: map[string]string{
					"b": "bool_flag",
					"j": "jobs",
					"o": "other_bool_flag",
				},
			}
			parser := BazelRcParser{
				workspaceDirectory: testDir,
				knownFlagData:      flagData,
			}

			cmd, err := parser.Parsefile(strings.NewReader(tc.input), "/sample/bazelrc")
			if tc.expectedErrorSubstring != "" {
				require.ErrorContains(t, err, tc.expectedErrorSubstring)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantOutput, cmd)
		})
	}
}

func TestIoReaderErrorReturnsErr(t *testing.T) {
	Parser := BazelRcParser{}
	Reader := &ErroringReader{}
	_, err := Parser.Parsefile(Reader, "/sample/bazelrc")
	require.EqualError(t, err, "failed to process /sample/bazelrc, failed to read file: always errors")
}

type ErroringReader struct{}

func (ErroringReader) Read(b []byte) (n int, err error) {
	return 0, fmt.Errorf("always errors")
}

func newFile(t *testing.T, directory, basename, contents string) string {
	absolutePath := filepath.Join(directory, basename)
	err := os.WriteFile(absolutePath, []byte(contents), 0o644)
	require.NoError(t, err)
	return absolutePath
}
