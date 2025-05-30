using System.IO;
namespace CRAMC.CryptoUtils;

public static class Crypter {
    // insecure encryption (just too lazy) using hardcoded password
    // nonce 12 bytes, tag 16 bytes, key 32 bytes, chacha20-poly1305
    // encrypted file = nonce 12 + tag 16 + ciphertext
    private static string _encPassword = "5bd67722e744501b8a5403daa793ff58b4dd4598a841bf4fe36e5cf0b67c4a48";
    
    // we encrypt/decrypt our configuration and tiny database, 
    // it shouldn't eat too much memory, so it's not necessary to
    // utilize stream for minimum memory footprint
    public static byte[] Chacha20Poly1305AEADEncrypt(string hexPassword, byte[] plainText) {

    }

    public static byte[] Chacha20Poly1305AEADDecrypt(string hexPassword, byte[] combinedCiphertext) {

    }
}