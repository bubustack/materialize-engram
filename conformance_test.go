package main

import (
	"testing"

	"github.com/bubustack/bubu-sdk-go/conformance"
	"github.com/bubustack/materialize-engram/pkg/config"
	"github.com/bubustack/materialize-engram/pkg/engram"
)

func TestConformance(t *testing.T) {
	suite := conformance.BatchSuite[config.Config, config.Inputs]{
		Engram: engram.New(),
		Config: config.Config{},
		Inputs: config.Inputs{},
	}
	suite.Run(t)
}
