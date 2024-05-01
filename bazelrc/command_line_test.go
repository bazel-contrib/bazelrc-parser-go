package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommandLineArgs(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []string
		want *CommandLineArgsAfterCommand
	}{
		"OneTarget": {
			in: []string{"//some:target"},
			want: &CommandLineArgsAfterCommand{
				BazelFlags: BazelFlagValues{},
				Targets:    []string{"//some:target"},
			},
		},
		"OneTargetWithBazelFlags": {
			in: []string{"--jobs=8", "--incompatible_strict_action_env", "@remote//some:target", "--remote_instance_name=blah"},
			want: &CommandLineArgsAfterCommand{
				BazelFlags: BazelFlagValues{
					"jobs":                           []string{"8"},
					"incompatible_strict_action_env": []string{"true"},
					"remote_instance_name":           []string{"blah"},
				},
				Targets: []string{"@remote//some:target"},
			},
		},
		"OneTargetWithBazelFlagsAndNonBazelFlags": {
			in: []string{"--jobs=8", "--incompatible_strict_action_env", "@remote//some:target", "--remote_instance_name=blah", "--", "--flag_for_target", "--other_flag_for_target", "positional"},
			want: &CommandLineArgsAfterCommand{
				BazelFlags: BazelFlagValues{
					"jobs":                           []string{"8"},
					"incompatible_strict_action_env": []string{"true"},
					"remote_instance_name":           []string{"blah"},
				},
				Targets:        []string{"@remote//some:target"},
				ExecutableArgs: []string{"--flag_for_target", "--other_flag_for_target", "positional"},
			},
		},
		"OneTargetWithBazelFlagsAndNonBazelFlagsAndBooleanFlagBeforeDoubleDash": {
			in: []string{"--jobs=8", "--incompatible_strict_action_env", "@remote//some:target", "--remote_instance_name=blah", "--verbose_failures", "--", "--flag_for_target", "--other_flag_for_target", "--jobs=8", "positional"},
			want: &CommandLineArgsAfterCommand{
				BazelFlags: BazelFlagValues{
					"jobs":                           []string{"8"},
					"incompatible_strict_action_env": []string{"true"},
					"remote_instance_name":           []string{"blah"},
					"verbose_failures":               []string{"true"},
				},
				Targets:        []string{"@remote//some:target"},
				ExecutableArgs: []string{"--flag_for_target", "--other_flag_for_target", "--jobs=8", "positional"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			flagData := &FlagData{
				BooleanFlags: map[string]bool{
					"incompatible_strict_action_env": true,
					"verbose_failures":               true,
				},
			}
			got, err := ParseCommandLineArgsAfterCommand(flagData, tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
