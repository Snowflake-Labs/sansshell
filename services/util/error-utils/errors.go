/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

package error_utils

import "fmt"

// ErrorWithCode defines a custom error type with a code.
type ErrorWithCode[T comparable] interface {
	error
	Code() T
}

// NewErrorWithCode creates a new ErrorWithCode with the given code and message.
func NewErrorWithCode[T comparable](code T, message string) ErrorWithCode[T] {
	return &errorWithCode[T]{
		code:    code,
		Message: message,
	}
}

// NewErrorWithCodef creates a new ErrorWithCode with the given code and formatted message.
func NewErrorWithCodef[T comparable](code T, format string, args ...any) ErrorWithCode[T] {
	return NewErrorWithCode[T](code, fmt.Sprintf(format, args...))
}

// errorWithCode implements the ErrorWithCode interface.
type errorWithCode[T comparable] struct {
	code    T
	Message string
}

// Error implements the error interface for ErrorWithCode.
func (e *errorWithCode[T]) Error() string {
	return fmt.Sprintf("[%v]: %s", e.code, e.Message)
}

// Code implements the [ErrorWithCode.Code] method.
func (e *errorWithCode[T]) Code() T {
	return e.code
}

// String implements the [fmt.Stringer] interface for ErrorWithCode.
func (e *errorWithCode[T]) String() string {
	return fmt.Sprintf("[%s] %s", e.code, e.Message)
}
