package mpahooks

import (
	"errors"
	"testing"
)

func TestIsUnqualifiedMPAApproval(t *testing.T) {
	err := errors.New("Authz policy does not permit this request: MPA_APPROVAL_UNQUALIFIED: [sansshell/ops] Approver must elevate")
	if !isUnqualifiedMPAApproval(err) {
		t.Fatal("expected unqualified MPA approval detection")
	}
	if isUnqualifiedMPAApproval(errors.New("permission denied")) {
		t.Fatal("did not expect generic permission denied to match")
	}
}
