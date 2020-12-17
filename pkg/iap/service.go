package iap

import (
	"fmt"

	androidService "bitbucket.org/calmisland/go-server-iap-platform/android"
	iosService "bitbucket.org/calmisland/go-server-iap-platform/ios"
)

// Service ...
type Service struct {
	IosSharedSecrects map[string]string
	AndroidPublicKeys map[string]string
}

var (
	instance *Service
)

// GetService ...
func GetService() *Service {
	if instance == nil {
		instance = &Service{} // <-- not thread safe
		instance.AndroidPublicKeys = make(map[string]string)
		instance.IosSharedSecrects = make(map[string]string)
	}

	return instance
}

// Initialize ..
func (service *Service) Initialize() error {
	androidList, err := androidService.GetService().GetAndroidList()

	if err != nil {
		return fmt.Errorf("could not load android information from db: %w", err)
	}

	for _, v := range androidList {
		service.AndroidPublicKeys[v.ApplicationID] = v.PublicKey
		// fmt.Printf("%s - %s \n", v.ApplicationID, v.PublicKey)
	}

	iosList, err := iosService.GetService().GetIosList()

	if err != nil {
		return fmt.Errorf("could not load ios information from db: %w", err)
	}

	for _, v := range iosList {
		service.IosSharedSecrects[v.BundelID] = v.SharedSecret
		// fmt.Printf("%s - %s \n", v.BundelID, v.SharedSecret)
	}

	fmt.Println("ios, android information is loaded successfully from db")
	return nil
}

// GetAndroidPublicKey ...
func (service *Service) GetAndroidPublicKey(appID string) string {
	return service.AndroidPublicKeys[appID]
}

// GetIosSharedKey ...
func (service *Service) GetIosSharedKey(bundleID string) string {
	return service.IosSharedSecrects[bundleID]
}
