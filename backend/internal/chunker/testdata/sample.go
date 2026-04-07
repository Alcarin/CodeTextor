package calculator

import (
	"fmt"
	"math"
)

import "os"

// MAX_PRECISION defines the maximum decimal precision.
const MAX_PRECISION = 15

// defaultRounding holds the default rounding mode.
var defaultRounding = "half_up"

// Calculator provides arithmetic operations.
type Calculator struct {
	precision int
}

// Computable defines an interface for computable objects.
type Computable interface {
	Compute() float64
}

// NewCalculator creates a calculator with the given precision.
func NewCalculator(precision int) *Calculator {
	return &Calculator{precision: precision}
}

// Add returns the sum of a and b.
func (c *Calculator) Add(a, b float64) float64 {
	// TODO: handle overflow
	return a + b
}

// Subtract returns the difference of a and b.
func (c *Calculator) Subtract(a, b float64) float64 {
	return a - b
}

// Sqrt returns the square root of x using the math package.
func Sqrt(x float64) float64 {
	// FIXME: handle negative input
	return math.Sqrt(x)
}

// Format returns a formatted string for the result.
func Format(value float64) string {
	return fmt.Sprintf("%.2f", value)
}
