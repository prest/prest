// Copyright 2011 Aaron Jacobs. All Rights Reserved.
// Author: aaronjjacobs@gmail.com (Aaron Jacobs)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oglematchers_test

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/assertions/internal/oglematchers"
	. "github.com/smartystreets/assertions/internal/ogletest"
)

func TestPassingTest(t *testing.T) { RunTests(t) }

////////////////////////////////////////////////////////////////////////
// PassingTest
////////////////////////////////////////////////////////////////////////

type PassingTest struct {
}

func init() { RegisterTestSuite(&PassingTest{}) }

func (t *PassingTest) EmptyTestMethod() {
}

func (t *PassingTest) SuccessfullMatches() {
	ExpectThat(17, Equals(17.0))
	ExpectThat(16.9, LessThan(17))
	ExpectThat("taco", HasSubstr("ac"))

	AssertThat(17, Equals(17.0))
	AssertThat(16.9, LessThan(17))
	AssertThat("taco", HasSubstr("ac"))
}

func (t *PassingTest) ExpectAliases() {
	ExpectEq(17, 17.0)

	ExpectLe(17, 17.0)
	ExpectLe(17, 18.0)
	ExpectLt(17, 18.0)

	ExpectGe(17, 17.0)
	ExpectGe(17, 16.0)
	ExpectGt(17, 16.0)

	ExpectNe(17, 18.0)

	ExpectTrue(true)
	ExpectFalse(false)
}

func (t *PassingTest) AssertAliases() {
	AssertEq(17, 17.0)

	AssertLe(17, 17.0)
	AssertLe(17, 18.0)
	AssertLt(17, 18.0)

	AssertGe(17, 17.0)
	AssertGe(17, 16.0)
	AssertGt(17, 16.0)

	AssertNe(17, 18.0)

	AssertTrue(true)
	AssertFalse(false)
}

func (t *PassingTest) SlowTest() {
	time.Sleep(37 * time.Millisecond)
}

////////////////////////////////////////////////////////////////////////
// PassingTestWithHelpers
////////////////////////////////////////////////////////////////////////

type PassingTestWithHelpers struct {
}

var _ SetUpTestSuiteInterface = &PassingTestWithHelpers{}
var _ SetUpInterface = &PassingTestWithHelpers{}
var _ TearDownInterface = &PassingTestWithHelpers{}
var _ TearDownTestSuiteInterface = &PassingTestWithHelpers{}

func init() { RegisterTestSuite(&PassingTestWithHelpers{}) }

func (t *PassingTestWithHelpers) SetUpTestSuite() {
	fmt.Println("SetUpTestSuite ran.")
}

func (t *PassingTestWithHelpers) SetUp(ti *TestInfo) {
	fmt.Println("SetUp ran.")
}

func (t *PassingTestWithHelpers) TearDown() {
	fmt.Println("TearDown ran.")
}

func (t *PassingTestWithHelpers) TearDownTestSuite() {
	fmt.Println("TearDownTestSuite ran.")
}

func (t *PassingTestWithHelpers) EmptyTestMethod() {
}
