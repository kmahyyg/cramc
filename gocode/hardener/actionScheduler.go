package hardener

import (
	"cramc_go/common"
	"cramc_go/customerrs"
)

func DispatchHardenAction(hAction *common.HardeningAction) error {
	if common.DryRunOnly {
		common.Logger.Infof("DryRun Set, No action, Information received: %v \n", hAction)
		return nil
	}
	if !common.IsRunningOnWin {
		return customerrs.ErrUnsupportedPlatform
	}
	common.Logger.Infoln("DEBUG: About to acquire HardenedDetectionTypesLock for: ", hAction.Name)
	
	// Check if already hardened and decide whether to proceed
	common.HardenedDetectionTypesLock.Lock()
	common.Logger.Infoln("DEBUG: Successfully acquired HardenedDetectionTypesLock for: ", hAction.Name)
	_, ok := common.HardenedDetectionTypes[hAction.Name]
	shouldProceed := true
	if ok && !hAction.AllowRepeatedHarden {
		shouldProceed = false
		common.Logger.Infoln("This detection does NOT allow repeated hardening action. Continue.")
	} else {
		common.HardenedDetectionTypes[hAction.Name] = true
	}
	common.HardenedDetectionTypesLock.Unlock()
	common.Logger.Infoln("DEBUG: Released HardenedDetectionTypesLock for: ", hAction.Name)
	
	if !shouldProceed {
		return nil
	}
	
	// Perform hardening action outside of mutex lock
	takeProperHardenAction(hAction)
	return nil
}

func takeProperHardenAction(hAction *common.HardeningAction) {
	// current supported:
	//    file: clean_setRO
	//     dir: setRO
	// action based on needs.
	for _, act := range hAction.ActionLst {
		fStr, err := applyTextTemplate(act.Dest)
		if err != nil {
			common.Logger.Errorln(err)
			continue
		}
		switch act.Action {
		case "clean_setRO":
			f_harden_CleanSetRO(act.Type, fStr)
		case "setRO":
			f_harden_SetRO(act.Type, fStr)
		default:
			common.Logger.Warnln("Unsupported action type: ", act.Action)
		}
	}
	return
}
