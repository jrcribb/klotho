package aws

import (
	"regexp"

	"github.com/klothoplatform/klotho/pkg/sanitization"
)

// LambdaFunctionSanitizer returns a sanitized lambda function name when applied.
var LambdaFunctionSanitizer = sanitization.NewSanitizer(
	[]sanitization.Rule{
		// strip any leading non alpha characters
		{
			Pattern:     regexp.MustCompile(`^[^a-zA-Z]+`),
			Replacement: "",
		},
		// replace "-" or whitespace with "_"
		{
			Pattern:     regexp.MustCompile(`[-\s]+`),
			Replacement: "_",
		},
		// strip any other invalid characters
		{
			Pattern:     regexp.MustCompile(`[^a-zA-Z0-9_]+`),
			Replacement: "",
		},
	}, 64)

// EnvVarKeySanitizer returns a sanitized environment key when applied.
var LambdaPermissionSanitizer = sanitization.NewSanitizer(
	[]sanitization.Rule{
		// strip any leading non alpha characters
		{
			Pattern:     regexp.MustCompile(`^[^a-zA-Z]+`),
			Replacement: "",
		},
		// replace "-" or whitespace with "_"
		{
			Pattern:     regexp.MustCompile(`[-\s]+`),
			Replacement: "_",
		},
		// strip any other invalid characters
		{
			Pattern:     regexp.MustCompile(`[^a-zA-Z0-9_]+`),
			Replacement: "",
		},
	}, 64)