package deadcond_test

import (
	"testing"

	"github.com/gostaticanalysis/exp/deadcond"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, deadcond.Analyzer, "a")
}
