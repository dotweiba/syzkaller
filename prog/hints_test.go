package prog

import (
	_ "encoding/binary"
	"fmt"
	"reflect"
	"sort"
	"testing"
)

var (
	simpleProgText = "syz_test$simple_test_call(0x%x)\n"
)

func TestHints(t *testing.T) {
	type Test struct {
		name  string
		in    uint64
		comps CompMap
		res   []uint64
	}
	var tests = []Test{
		{
			"Dumb test",
			0xdeadbeef,
			CompMap{0xdeadbeef: uint64Set{0xcafebabe: true}},
			[]uint64{0xcafebabe},
		},
		// Test for cases when there's multiple comparisons (op1, op2), (op1, op3), ...
		// Checks that for every such operand a program is generated.
		{
			0xabcd,
			CompMap{0xabcd: uint64Set{0x1: true, 0x2: true, 0x3: true}},
			[]uint64{0x1, 0x2, 0x4},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.name), func(t *testing.T) {
			sort.Slice(test.res, func(i, j) bool {
				return test.res[i] < test.res[j]
			})
			res := getReplacersForVal(test.in, test.comps)
			if !reflect.DeepEqual(res, test.res) {
				t.Fatalf("got : %v\nwant: %v", got, want)
			}
		})
	}
}

func TestHintsData(t *testing.T) {
	type Test struct {
		in    string
		comps CompMap
		res   []string
	}
	var tests = []Test{
		// Dumb test.
		{
			"abcdef",
			CompMap{"cd": uint64Set{"42": true}},
			[]string{"ab42ef"},
		},
		{
			"\xde\xad\xbe\xef\x44\x45",
			CompMap{0xdead: uint64Set{"42": true}},
			[]string{"ab42ef"},
		},
		{
			[]byte{0x01, 0x02, 0x01, 0x02, 0x01, 0x02},
			CompMap{[]byte{0x01, 0x02}: uint64Set{[]byte{0x08, 0x09}: true}},
			[][]byte{[]byte{0x08, 0x09, 0x01, 0x02, 0x01, 0x02}},
		},
		// Test for cases when there's multiple comparisons (op1, op2), (op1, op3), ...
		// Checks that for every such operand a program is generated.
		{
			0xabcd,
			CompMap{0xabcd: uint64Set{0x1: true, 0x2: true, 0x3: true}},
			[]uint64{0x1, 0x2, 0x4},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%+v", test), func(t *testing.T) {
			p, _ := Deserialize([]byte(getSimpleProgText(test.in)))
			var got []string
			p.MutateWithHints([]CompMap{test.comps}, func(newP *Prog) {
				got = append(got, string(newP.Serialize()))
			})
			var want []string
			for _, res := range test.res {
				want = append(want, getSimpleProgText(res))
			}
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got : %v\nwant: %v", got, want)
			}
		})
	}
}

func testCommon(t *testing.T, p *prog, want []string, comps CompMap) {
	var got []string
	p.MutateWithHints([]CompMap{test.comps}, func(newP *Prog) {
		got = append(got, string(newP.Serialize()))
	})
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got : %v\nwant: %v", got, want)
	}
}

func TestHintsConstArgShrinkSize(t *testing.T) {
	m := CompMap{
		0xab: uint64Set{0x1: true},
	}
	expected := []string{
		getSimpleProgText(0x1),
	}

	// Code for positive values - drop the trash from highest bytes.
	runSimpleTest(m, expected, t, 0x12ab)
	runSimpleTest(m, expected, t, 0x123456ab)
	runSimpleTest(m, expected, t, 0x1234567890abcdab)

	// Code for negative values - drop the 0xff.. prefix
	runSimpleTest(m, expected, t, 0xffab)
	runSimpleTest(m, expected, t, 0xffffffab)
	runSimpleTest(m, expected, t, 0xffffffffffffffab)
}

// Test for cases, described in expandMutation() function.
func TestHintsConstArgExpandSize(t *testing.T) {
	m := CompMap{
		0xffffffffffffffab: uint64Set{0x1: true},
	}
	expected := []string{
		getSimpleProgText(0x1),
	}
	runSimpleTest(m, expected, t, 0xab)
	runSimpleTest(m, expected, t, 0xffab)
	runSimpleTest(m, expected, t, 0xffffffab)

	m = CompMap{
		0xffffffab: uint64Set{0x1: true},
	}
	expected = []string{
		getSimpleProgText(0x1),
	}
	runSimpleTest(m, expected, t, 0xab)
	runSimpleTest(m, expected, t, 0xffab)

	m = CompMap{
		0xffab: uint64Set{0x1: true},
	}
	expected = []string{
		getSimpleProgText(0x1),
	}
	runSimpleTest(m, expected, t, 0xab)
}

// Helper functions.
func byteArraysDifferent(a, b []byte) bool {
	if len(a) != len(b) {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

func getSimpleProgText(a uint64) string {
	return fmt.Sprintf(simpleProgText, a)
}

func runSimpleTest(m CompMap, expected []string, t *testing.T, a uint64) {
	runTest(m, expected, t, getSimpleProgText(a))
}

func runTest(m CompMap, expected []string, t *testing.T, progText string) {
	p, _ := Deserialize([]byte(progText))
	got := make([]string, 0)
	f := func(newP *Prog) {
		got = append(got, string(newP.Serialize()))
	}
	p.MutateWithHints([]CompMap{m}, f)
	sort.Strings(got)
	sort.Strings(expected)
	if len(got) != len(expected) {
		t.Fatal("Lengths of got and expected differ", "got", got,
			"expected", expected)
	}
	failed := false
	for i := range expected {
		if expected[i] != got[i] {
			failed = true
			break
		}
	}
	if failed {
		t.Error("Got", got, "expected", expected)
	}
}
