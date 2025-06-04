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
	common.HardenedDetectionTypesLock.Lock()
	_, ok := common.HardenedDetectionTypes[hAction.Name]
	if ok {
		if !hAction.AllowRepeatedHarden {
			common.Logger.Infoln("This detection does NOT allow repeated hardening action. Continue.")
			return nil
		}
	}
	common.HardenedDetectionTypes[hAction.Name] = true
	takeProperHardenAction(hAction)
	common.HardenedDetectionTypesLock.Unlock()
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
