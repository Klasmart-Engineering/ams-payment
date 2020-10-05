package iap

import (
	"encoding/json"
	"testing"

	"github.com/awa/go-iap/playstore"
)

type playStoreReceipt struct {
	JSON      string `json:"json"`
	Signature string `json:"signature"`
}

type playStoreFullReceipt struct {
	Payload string `json:"Payload"`
}

func TestMakeHashTableAndroidAndVerify(t *testing.T) {
	GetService().Initialize()

	publicKey := GetService().GetAndroidPublicKey("com.calmid.learnandplay.launcher")

	receipt := []byte("{\"Store\":\"GooglePlay\",\"TransactionID\":\"GPA.3309-5713-9890-58987\",\"Payload\":\"{\\\"json\\\":\\\"{\\\\\\\"orderId\\\\\\\":\\\\\\\"GPA.3309-5713-9890-58987\\\\\\\",\\\\\\\"packageName\\\\\\\":\\\\\\\"com.calmid.learnandplay.launcher\\\\\\\",\\\\\\\"productId\\\\\\\":\\\\\\\"com.calmid.badanamu.esl.premium\\\\\\\",\\\\\\\"purchaseTime\\\\\\\":1600760196200,\\\\\\\"purchaseState\\\\\\\":0,\\\\\\\"developerPayload\\\\\\\":\\\\\\\"{\\\\\\\\\\\\\\\"developerPayload\\\\\\\\\\\\\\\":\\\\\\\\\\\\\\\"\\\\\\\\\\\\\\\",\\\\\\\\\\\\\\\"is_free_trial\\\\\\\\\\\\\\\":false,\\\\\\\\\\\\\\\"has_introductory_price_trial\\\\\\\\\\\\\\\":false,\\\\\\\\\\\\\\\"is_updated\\\\\\\\\\\\\\\":false,\\\\\\\\\\\\\\\"accountId\\\\\\\\\\\\\\\":\\\\\\\\\\\\\\\"\\\\\\\\\\\\\\\"}\\\\\\\",\\\\\\\"purchaseToken\\\\\\\":\\\\\\\"gekbpaobmklikbiphbnadeel.AO-J1OwprsKk1PwLrYJm48X9dHK1qRTuoR7UHKLWtvlrk4lvYn1KFewJsx2-rTjerDh7bRr8w8D5u8-dwSH3wVLjm6GCL3Nh_MkVwpOQJ1ho__nZswMHf-sELQvm55-xMw4ellQTShglbXdtk5yg_rNiWVkAhcEuMnmh4aQTXww5rLrfRWeCOwk\\\\\\\"}\\\",\\\"signature\\\":\\\"Bt7Nzxx9tA\\\\/38+EQjUD1PqBbx5JajBFmtls6ekAbzxkLL0bOvZTyFM0IZQY\\\\/J6JOKUVFeLJ+K8W5UEPooxlxhE3K9Hakl+fqLkNYxbd8\\\\/wrbsDKQpWzeDVyfzGimuoVr3xc2Cm235ov3HVHcV6qTWFQW0dn0icr+xHL2dADfqZc2Xv51cA+yT9\\\\/qgNhi5jUsmdcK7oHtdexqbB+24V7IbqpGTQG9hi+VFBcOF0awgGRj9ktmYx6MLuHzX41mf9RwW0Z\\\\/epISVGm5yGIgF7flvmyj0502CkKYOZhGLBBNp6mLyXS3TojqtHAEUgSLo9KICuM5\\\\/OBcayZOVbLIG6uvbg==\\\",\\\"skuDetails\\\":\\\"{\\\\\\\"skuDetailsToken\\\\\\\":\\\\\\\"AEuhp4KIf25int8_McJRHB613wT2GesbSHHKdNnsogsBx9NaUu0ocUlIgfqMmd-lJfi9\\\\\\\",\\\\\\\"productId\\\\\\\":\\\\\\\"com.calmid.badanamu.esl.premium\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"inapp\\\\\\\",\\\\\\\"price\\\\\\\":\\\\\\\"₩46,000\\\\\\\",\\\\\\\"price_amount_micros\\\\\\\":46000000000,\\\\\\\"price_currency_code\\\\\\\":\\\\\\\"KRW\\\\\\\",\\\\\\\"title\\\\\\\":\\\\\\\"Badanamu Leraning Pass Premium (Badanamu: Badanamu ESL™)\\\\\\\",\\\\\\\"description\\\\\\\":\\\\\\\"Unlock all contents in Badanamu ESL App\\\\\\\"}\\\",\\\"isPurchaseHistorySupported\\\":true}\"}")

	var full playStoreFullReceipt
	err := json.Unmarshal(receipt, &full)

	if err != nil {
		t.Errorf(err.Error())
	}

	var receiptInfo playStoreReceipt
	err = json.Unmarshal([]byte(full.Payload), &receiptInfo)

	if err != nil {
		t.Errorf(err.Error())
	}

	isValid, err := playstore.VerifySignature(publicKey, []byte(receiptInfo.JSON), receiptInfo.Signature)

	if err != nil {
		t.Errorf(err.Error())
	}

	if isValid == false {
		t.Errorf("VerifySignature failed")
	}

}
