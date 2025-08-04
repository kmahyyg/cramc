# Encrypted File Info

## Encryption Key

Encryption Key = [here](gocode/common/shared.go#L29) , in hex: `1928da3545b48068e024d06f2f132c728eabcd933a8659e578d7a82fde0cd948`

This is only a weak protection to minimize possibility for being killed by antivirus in wrong way.

## Encryption Details

Encryption Methods: XChacha20-Poly1305 (AEAD)

### Special note for associated data

For `.bin` files which are configuration/necessary supplemental file for program:

Program will append file original path as part of associated data: [compiled yara rules](assets/build.sh#L82) and [database](assets/build.sh#L83)

For other files that are generated during runtime, file original path will be appended as suffix of associated data.

### General layout

```
|  24 bytes  |        4 bytes        |       4 bytes           |            X bytes             |     Y bytes     |  16 bytes |
|     IV     |                                  Associated Data                                 |    Ciphertext   |  AEAD Tag |
| IV (Nonce) |  KCRC32 of plaintext  | length of original path | original length (byte, UTF-8)  |  Encrypted data |  AEAD Tag |
```