package patterns

import (
    "fmt"
    "testing"
)

func TestFSILValidation(t *testing.T) {
    // Test the prefix validation
    codes := []string{"FSIL", "FCQD", "FSIA", "FEFF", "CYVR", "KJFK", "YSSY"}
    for _, code := range codes {
        valid := IsValidICAO(code)
        prefix := ""
        if len(code) >= 2 {
            prefix = code[:2]
        }
        fmt.Printf("%s (prefix=%s): valid=%v, hasValidPrefix=%v\n", 
            code, prefix, valid, hasValidICAOPrefix(code))
    }
}
