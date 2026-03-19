package hardener

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/telemetry"
	"fmt"
)

func DispatchHardenAction(hAction *common.HardeningAction) error {
	if common.DryRunOnly {
		common.Logger.Info(fmt.Sprintf("DryRun Set, No action, Information received: %v ", hAction))
		return nil
	}
	if !common.IsRunningOnWin {
		return customerrs.ErrUnsupportedPlatform
	}
	common.Logger.Debug("DEBUG: About to acquire HardenedDetectionTypesLock for: " + hAction.Name)

	// Check if already hardened and decide whether to proceed
	common.HardenedDetectionTypesLock.Lock()
	common.Logger.Debug("DEBUG: Successfully acquired HardenedDetectionTypesLock for: " + hAction.Name)
	_, ok := common.HardenedDetectionTypes[hAction.Name]
	shouldProceed := true
	if ok && !hAction.AllowRepeatedHarden {
		shouldProceed = false
		common.Logger.Info("This detection does NOT allow repeated hardening action. Continue.")
	} else {
		common.HardenedDetectionTypes[hAction.Name] = true
	}
	common.HardenedDetectionTypesLock.Unlock()
	common.Logger.Debug("DEBUG: Released HardenedDetectionTypesLock for: " + hAction.Name)

	if !shouldProceed {
		return nil
	}

	// Perform hardening action outside of mutex lock
	takeProperHardenAction(hAction)
	return nil
}

func takeProperHardenAction(hAction *common.HardeningAction) {
	// current supported:
	//    file & dir: clean_setRO, setRO
	//     file only: rm_replaceDir_setRO
	//      dir only: replaceFile_setRO
	// action based on needs.
	for _, act := range hAction.ActionLst {
		fStr, err := applyTextTemplate(act.Dest)
		if err != nil {
			common.Logger.Error(err.Error())
			continue
		}
		switch act.Action {
		case "replaceFile_setRO":
			f_harden_replaceFileSetRO(act.Type, fStr)
		case "rm_replaceDir_setRO":
			f_harden_rmReplaceDirSetRO(act.Type, fStr)
		case "clean_setRO":
			f_harden_CleanSetRO(act.Type, fStr)
		case "setRO":
			f_harden_SetRO(act.Type, fStr)
		default:
			telemetry.CaptureMessage("error", "Unsupported hardening action type: "+act.Action)
			common.Logger.Warn("Unsupported action type: " + act.Action)
		}
	}
	return
}
