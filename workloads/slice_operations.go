// Package workloads is the terraform provider
package workloads

// Contains check if a slice contains an element
func Contains[T comparable](elements []T, element T) bool {
	for _, e := range elements {
		if element == e {
			return true
		}
	}
	return false
}

// Delete removes an element from a slice
func Delete[T comparable](elements []T, element T) []T {
	for i, v := range elements {
		if v == element {
			elements = append(elements[:i], elements[i+1:]...)
			break
		}
	}
	return elements
}
