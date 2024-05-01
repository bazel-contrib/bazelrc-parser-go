package bazelrc

// BazelrcContents holds the output of parsing a Bazelrc file.
type BazelrcContents struct {
	// entries is a map of commands (e.g. "build") to flag names without leading dashes (e.g. "action_env")
	// to values (in the order they were encountered in the file).
	entries map[string]BazelFlagValues
}

type BazelFlagValues map[string][]string

// FlagValue gets the effective (i.e. last) value for a particular flag.
// It has no awareness of what flags are allowed multiple values, or default values.
func (c BazelFlagValues) FlagValue(flagname string) *string {
	values, ok := c[flagname]
	if !ok {
		return nil
	}
	lastValue := values[len(values)-1]
	return &lastValue
}

// FlagValue gets the effective (i.e. last) value for a particular flag.
// It has no awareness of what flags are allowed multiple values, or default values.
func (c *BazelrcContents) FlagValue(command string, flagname string) *string {
	commandSpecificFlags, ok := c.entries[command]
	if !ok {
		return nil
	}
	return commandSpecificFlags.FlagValue(flagname)
}

func newBazelrcContents() *BazelrcContents {
	return &BazelrcContents{
		entries: make(map[string]BazelFlagValues),
	}
}
