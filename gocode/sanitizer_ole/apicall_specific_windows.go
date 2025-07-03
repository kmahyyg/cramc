package sanitizer_ole

import (
	"syscall"
	"unsafe"
)

// -------- Excel COM API Call Constants Enum -------- //

type MsoAutomationSecurity int

const (
	MsoAutomationSecurityLow = 1 + iota
	MsoAutomationSecurityByUI
	MsoAutomationSecurityForceDisable
)

type XlCalculation int

const (
	XlCalculationManual        = -4135
	XlCalculationAutomatic     = -4105
	XlCalculationSemiautomatic = 2
)

type XlUpdateLinks int

const (
	XlUpdateLinksUserSetting = 1 + iota
	XlUpdateLinksNever
	XlUpdateLinksAlways
)

// -------- Windows COM API Impersonation Enum -------- //

type RpcCallAuthenticationLevel uint32

const (
	RPC_C_AUTHN_LEVEL_DEFAULT RpcCallAuthenticationLevel = iota
	RPC_C_AUTHN_LEVEL_NONE
	RPC_C_AUTHN_LEVEL_CONNECT
	RPC_C_AUTHN_LEVEL_CALL
	RPC_C_AUTHN_LEVEL_PKT
	RPC_C_AUTHN_LEVEL_PKT_INTEGRITY
	RPC_C_AUTHN_LEVEL_PKT_PRIVACY
)

type RpcCallImpersonationLevel uint32

const (
	RPC_C_IMP_LEVEL_DEFAULT RpcCallImpersonationLevel = iota
	RPC_C_IMP_LEVEL_ANONYMOUS
	RPC_C_IMP_LEVEL_IDENTIFY
	RPC_C_IMP_LEVEL_IMPERSONATE
	RPC_C_IMP_LEVEL_DELEGATE
)

// https://learn.microsoft.com/en-us/windows/win32/api/objidl/ne-objidl-eole_authentication_capabilities
type EOLEAuthenticationCapabilities uint32

const (
	EOAC_NONE              EOLEAuthenticationCapabilities = 0
	EOAC_MUTUAL_AUTH                                      = 1
	EOAC_STATIC_CLOAKING                                  = 0x20
	EOAC_DYNAMIC_CLOAKING                                 = 0x40
	EOAC_ANY_AUTHORITY                                    = 0x80
	EOAC_MAKE_FULLSIC                                     = 0x100
	EOAC_DEFAULT                                          = 0x800
	EOAC_SECURE_REFS                                      = 0x2
	EOAC_ACCESS_CONTROL                                   = 0x4
	EOAC_APPID                                            = 0x8
	EOAC_DYNAMIC                                          = 0x10
	EOAC_REQUIRE_FULLSIC                                  = 0x200
	EOAC_AUTO_IMPERSONATE                                 = 0x400
	EOAC_DISABLE_AAA                                      = 0x1000
	EOAC_NO_CUSTOM_MARSHAL                                = 0x2000
	EOAC_RESERVED1                                        = 0x4000
)

type HRESULT int32

// SOLEAuthenticationService: https://learn.microsoft.com/en-us/windows/win32/api/objidl/ns-objidl-sole_authentication_service
type SOLEAuthenticationService struct {
	DwAuthnSvc     uint32
	DwAuthzSvc     uint32
	PPrincipalName *uint16 //OLECHAR* (utf-16 string)
	Hr             HRESULT
}

type SOLEAuthenticationInfo struct {
	DwAuthnSvc uint32
	DwAuthzSvc uint32
	PAuthInfo  uintptr
}

// CoInitializeSecurity from MSDN
// https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-coinitializesecurity
// why I need this? https://learn.microsoft.com/en-us/windows/win32/com/setting-processwide-security-with-coinitializesecurity
//
// Data Type: https://learn.microsoft.com/en-us/cpp/cpp/data-type-ranges?view=msvc-170
// https://learn.microsoft.com/en-us/windows/win32/winprog/windows-data-types
// https://learn.microsoft.com/en-us/windows/win32/seccrypto/common-hresult-values
//
// HRESULT CoInitializeSecurity(
//
//	[in, optional] PSECURITY_DESCRIPTOR        pSecDesc,  // PVOID
//	[in]           LONG                        cAuthSvc,  // uint32, count of asAuthSvc
//	[in, optional] SOLE_AUTHENTICATION_SERVICE *asAuthSvc,  // struct
//	[in, optional] void                        *pReserved1,
//	[in]           DWORD                       dwAuthnLevel,
//	[in]           DWORD                       dwImpLevel,
//	[in, optional] void                        *pAuthList,  // a pointer to SOLE_AUTHENTICATION_LIST, is an array of SOLEAuthenticationInfo
//	[in]           DWORD                       dwCapabilities,
//	[in, optional] void                        *pReserved3
//
// );
func CoInitializeSecurity(psecDesc uintptr, cAuthsvc *int32, asAuthSvc *SOLEAuthenticationService,
	pReserved1 uintptr, dwAuthnLevel RpcCallAuthenticationLevel, dwImpLevel RpcCallImpersonationLevel,
	pAuthList **SOLEAuthenticationInfo, dwCapabilities EOLEAuthenticationCapabilities,
	pReserved3 uintptr) error {
	ret, _, _ := syscall.SyscallN(procCoInitializeSecurity.Addr(), 9,
		psecDesc, uintptr(unsafe.Pointer(cAuthsvc)), uintptr(unsafe.Pointer(asAuthSvc)),
		nullptr, uintptr(dwAuthnLevel), uintptr(dwImpLevel), uintptr(unsafe.Pointer(pAuthList)),
		uintptr(dwCapabilities), nullptr)
	if uint32(ret) == 0 {
		// S_OK, Operation Successful == 0
		return nil
	}
	return syscall.Errno(ret)
}
