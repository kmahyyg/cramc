using System;
using System.IO;
using System.Security.Cryptography;
using Serilog;
using System.Text;

namespace CRAMC.CryptoUtils;

public static class Crypter {
    // insecure encryption (just too lazy) using hardcoded password
    // nonce 12 bytes, tag 16 bytes, key 32 bytes, chacha20-poly1305
    // encrypted file = nonce 12 + tag 16 + associatedData (CRC32 of plaintext) + ciphertext
    private static string _encPassword = "5bd67722e744501b8a5403daa793ff58b4dd4598a841bf4fe36e5cf0b67c4a48";
    
    // we encrypt/decrypt our configuration and tiny database,
    // it shouldn't require too much memory, so it's not necessary to
    // utilize stream for minimum memory footprint
    public static byte[] Chacha20Poly1305AEADEncrypt(string hexPassword, byte[] plainText) {
        try {
            // Convert hex password to bytes (32 bytes for ChaCha20)
            byte[] key = Convert.FromHexString(hexPassword);
            if (key.Length != 32) {
                Log.Error("Encryption key must be 32 bytes long");
                return Array.Empty<byte>();
            }

            // Generate random 12-byte nonce
            byte[] nonce = new byte[12];
            using (var rng = RandomNumberGenerator.Create()) {
                rng.GetBytes(nonce);
            }

            // Calculate CRC32 hash of plaintext as associated data
            byte[] associatedData = CalculateCrc32(plainText);

            // Encrypt using ChaCha20Poly1305
            using (var chacha = new ChaCha20Poly1305(key)) {
                byte[] ciphertext = new byte[plainText.Length];
                byte[] tag = new byte[16];
                
                chacha.Encrypt(nonce, plainText, ciphertext, tag, associatedData);
                
                // Combine: nonce (12) + tag (16) + associatedData (4) + ciphertext
                byte[] result = new byte[nonce.Length + tag.Length + associatedData.Length + ciphertext.Length];
                int offset = 0;
                
                Array.Copy(nonce, 0, result, offset, nonce.Length);
                offset += nonce.Length;
                
                Array.Copy(tag, 0, result, offset, tag.Length);
                offset += tag.Length;
                
                Array.Copy(associatedData, 0, result, offset, associatedData.Length);
                offset += associatedData.Length;
                
                Array.Copy(ciphertext, 0, result, offset, ciphertext.Length);
                
                return result;
            }
        }
        catch {
            return Array.Empty<byte>();
        }
    }

    public static byte[] Chacha20Poly1305AEADDecrypt(string hexPassword, byte[] combinedCiphertext) {
        try {
            // Minimum size check: nonce(12) + tag(16) + associatedData(4) = 32 bytes
            if (combinedCiphertext.Length < 32) {
                Log.Error("Encrypted data must be at least 32 bytes long");
                return Array.Empty<byte>();
            }

            // Convert hex password to bytes (32 bytes for ChaCha20)
            byte[] key = Convert.FromHexString(hexPassword);
            if (key.Length != 32) {
                Log.Error("Encryption key must be 32 bytes long");
                return Array.Empty<byte>();
            }

            // Extract components
            byte[] nonce = new byte[12];
            byte[] tag = new byte[16];
            byte[] associatedData = new byte[4];
            byte[] ciphertext = new byte[combinedCiphertext.Length - 32];

            int offset = 0;
            Array.Copy(combinedCiphertext, offset, nonce, 0, 12);
            offset += 12;
            
            Array.Copy(combinedCiphertext, offset, tag, 0, 16);
            offset += 16;
            
            Array.Copy(combinedCiphertext, offset, associatedData, 0, 4);
            offset += 4;
            
            Array.Copy(combinedCiphertext, offset, ciphertext, 0, ciphertext.Length);

            // Decrypt using ChaCha20Poly1305
            using (var chacha = new ChaCha20Poly1305(key)) {
                byte[] plaintext = new byte[ciphertext.Length];
                
                chacha.Decrypt(nonce, ciphertext, tag, plaintext, associatedData);
                
                // Validate CRC32 hash
                byte[] expectedCrc32 = CalculateCrc32(plaintext);
                if (!ArraysEqual(associatedData, expectedCrc32)) {
                    return Array.Empty<byte>();
                }
                
                return plaintext;
            }
        }
        catch {
            Log.Error("AEAD Decryption failed.");
            return Array.Empty<byte>();
        }
    }

    private static byte[] CalculateCrc32(byte[] data) {
        uint crc = 0xFFFFFFFF;
        uint[] table = GenerateCrc32Table();
        
        foreach (byte b in data) {
            crc = table[(crc ^ b) & 0xFF] ^ (crc >> 8);
        }
        
        crc ^= 0xFFFFFFFF;
        return BitConverter.GetBytes(crc);
    }

    private static uint[] GenerateCrc32Table() {
        uint[] table = new uint[256];
        uint polynomial = 0xEDB88320;
        
        for (uint i = 0; i < 256; i++) {
            uint crc = i;
            for (int j = 0; j < 8; j++) {
                if ((crc & 1) == 1) {
                    crc = (crc >> 1) ^ polynomial;
                } else {
                    crc >>= 1;
                }
            }
            table[i] = crc;
        }
        
        return table;
    }

    private static bool ArraysEqual(byte[] a, byte[] b) {
        if (a.Length != b.Length) return false;
        for (int i = 0; i < a.Length; i++) {
            if (a[i] != b[i]) return false;
        }
        return true;
    }
}