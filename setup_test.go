package textplain_test

import (
	"testing"

	"github.com/mailproto/textplain"
	"github.com/stretchr/testify/assert"
)

func runTestCases(t *testing.T, testCases []testCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			runTestCase(tt, tc)
		})
	}
}

func runTestCase(t *testing.T, tc testCase) {
	result, err := textplain.Convert(tc.body, textplain.DefaultLineLength)
	assert.Nil(t, err)
	assert.Equal(t, tc.expect, result)
}
