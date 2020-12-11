package android

import (
	"fmt"
	"testing"
)

func TestGetSubscriptionInformation(t *testing.T) {
	result, err := GetSubscriptionInformation("com.calmid.iap.tester", "com.calmid.inapp.tester.subscription.b", "jgjcjnhphianeclklbhjdocm.AO-J1Owv_oMGZaebVXNdJPp_6RFObeWeEsLMc2-UMai4rhRiAWWC8mcbnd_JqHuVlzObEKorQ8TwbzVmu5Cg591kTO3D9GsuIkeozBAUmlsFRmMsnJ-kNIc")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(result)
}
