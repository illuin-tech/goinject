package goinject

import "os"

type Conditional interface {
	evaluate() bool
}

type environmentVariableConditional struct {
	name           string
	havingValue    string
	matchIfMissing bool
}

func (c *environmentVariableConditional) evaluate() bool {
	val, ok := os.LookupEnv(c.name)
	if !ok {
		return c.matchIfMissing
	}
	return val == c.havingValue
}

func OnEnvironmentVariable(name, havingValue string, matchIfMissing bool) Conditional {
	return &environmentVariableConditional{
		name:           name,
		havingValue:    havingValue,
		matchIfMissing: matchIfMissing,
	}
}
