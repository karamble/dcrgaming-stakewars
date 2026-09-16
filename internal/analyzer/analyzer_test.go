package analyzer

import (
	"golang.org/x/tools/go/analysis/analysistest"
	"testing"
)

func TestBannedConstructs(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), New("simfixture"), "simfixture")
}
