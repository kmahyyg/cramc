package hardener

import "cramc_go/common"

func DispatchHardenAction(hAction *common.HardeningAction) error {
	if common.DryRunOnly {
		common.Logger.Infof("DryRun Set, No action, Information received: %v \n", hAction)
		return nil
	}
	common.HardenedDetectionTypesLock.Lock()
	_, ok := common.HardenedDetectionTypes[hAction.Name]
	if ok {
		if !hAction.AllowRepeatedHarden {
			common.Logger.Infoln("This detection does NOT allow repeated hardening action. Continue.")
			return nil
		}
	}
	common.HardenedDetectionTypes[hAction.Name] = true
	err := takeProperHardenAction(hAction)
	common.HardenedDetectionTypesLock.Unlock()
	return err
}

func takeProperHardenAction(hAction *common.HardeningAction) error {

}
