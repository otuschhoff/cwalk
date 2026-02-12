package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseInodeTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "single type",
			input:    "file",
			expected: map[string]bool{"file": true},
		},
		{
			name:     "multiple types",
			input:    "file,dir,symlink",
			expected: map[string]bool{"file": true, "dir": true, "symlink": true},
		},
		{
			name:     "types with spaces",
			input:    "file, dir , symlink",
			expected: map[string]bool{"file": true, "dir": true, "symlink": true},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]bool{"": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInodeTypes(tt.input)
			assert.Equal(t, len(tt.expected), len(result))
			for k := range tt.expected {
				assert.True(t, result[k], "missing key: %s", k)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(time.Duration) bool
	}{
		{
			name:    "days",
			input:   "7d",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == 7*24*time.Hour },
		},
		{
			name:    "weeks",
			input:   "2w",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == 2*7*24*time.Hour },
		},
		{
			name:    "minutes",
			input:   "30m",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == 30*time.Minute },
		},
		{
			name:    "hours",
			input:   "24h",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == 24*time.Hour },
		},
		{
			name:    "seconds",
			input:   "3600s",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == time.Hour },
		},
		{
			name:    "years",
			input:   "1y",
			wantErr: false,
			check:   func(d time.Duration) bool { return d == 365*24*time.Hour },
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "unknown unit",
			input:   "5x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.True(t, tt.check(result), "duration mismatch: got %v", result)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{
			name:     "bytes",
			input:    "1024",
			expected: 1024,
			wantErr:  false,
		},
		{
			name:     "kilobytes",
			input:    "1K",
			expected: 1024,
			wantErr:  false,
		},
		{
			name:     "kilobytes with B",
			input:    "1KB",
			expected: 1024,
			wantErr:  false,
		},
		{
			name:     "megabytes",
			input:    "1M",
			expected: 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "gigabytes",
			input:    "1G",
			expected: 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "terabytes",
			input:    "1T",
			expected: 1024 * 1024 * 1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "decimal value",
			input:    "1.5G",
			expected: int64(1.5 * 1024 * 1024 * 1024),
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "unknown unit",
			input:   "1X",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseStringList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single item",
			input:    "user1",
			expected: []string{"user1"},
		},
		{
			name:     "multiple items",
			input:    "user1,user2,user3",
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "items with spaces",
			input:    "user1 , user2 , user3",
			expected: []string{"user1", "user2", "user3"},
		},
		{
			name:     "empty items ignored",
			input:    "user1,,user3",
			expected: []string{"user1", "user3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStringList(tt.input)
			assert.Equal(t, len(tt.expected), len(result))
			for i, v := range result {
				assert.Equal(t, tt.expected[i], v)
			}
		})
	}
}

func TestParseUintList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []uint32
		wantErr  bool
	}{
		{
			name:     "single value",
			input:    "1000",
			expected: []uint32{1000},
			wantErr:  false,
		},
		{
			name:     "multiple values",
			input:    "1000,2000,3000",
			expected: []uint32{1000, 2000, 3000},
			wantErr:  false,
		},
		{
			name:     "values with spaces",
			input:    "1000 , 2000 , 3000",
			expected: []uint32{1000, 2000, 3000},
			wantErr:  false,
		},
		{
			name:     "empty values ignored",
			input:    "1000,,3000",
			expected: []uint32{1000, 3000},
			wantErr:  false,
		},
		{
			name:    "invalid value",
			input:   "1000,invalid,3000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseUintList(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expected), len(result))
				for i, v := range result {
					assert.Equal(t, tt.expected[i], v)
				}
			}
		})
	}
}

func TestParsePerms(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "user read",
			input:   "u+r",
			wantErr: false,
		},
		{
			name:    "group write",
			input:   "g+w",
			wantErr: false,
		},
		{
			name:    "other execute",
			input:   "o+x",
			wantErr: false,
		},
		{
			name:    "all bits",
			input:   "a+rwx",
			wantErr: false,
		},
		{
			name:    "multiple permissions",
			input:   "u+r,g+w,o+x",
			wantErr: false,
		},
		{
			name:    "invalid who",
			input:   "x+r",
			wantErr: true,
		},
		{
			name:    "invalid operator",
			input:   "u=r",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "u+",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePerms(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsDigit(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		expected bool
	}{
		{name: "zero", input: '0', expected: true},
		{name: "nine", input: '9', expected: true},
		{name: "five", input: '5', expected: true},
		{name: "letter", input: 'a', expected: false},
		{name: "space", input: ' ', expected: false},
		{name: "dot", input: '.', expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDigit(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
