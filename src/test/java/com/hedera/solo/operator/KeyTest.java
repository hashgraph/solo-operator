package com.hedera.solo.operator;

import com.swirlds.common.crypto.internal.CryptoUtils;
import com.swirlds.common.platform.NodeId;
import com.swirlds.platform.crypto.CryptoConstants;
import com.swirlds.platform.crypto.CryptoStatic;
import com.swirlds.platform.crypto.KeysAndCerts;
import com.swirlds.platform.system.address.AddressBookUtils;
import java.io.ByteArrayOutputStream;
import java.io.FileOutputStream;
import java.io.OutputStream;
import java.io.PrintStream;
import java.io.StringWriter;
import java.nio.ByteBuffer;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.SecureRandom;
import java.security.Security;
import java.security.cert.X509Certificate;
import java.util.Base64;
import java.util.Map;
import org.bouncycastle.asn1.pkcs.PrivateKeyInfo;
import org.bouncycastle.asn1.x509.SubjectPublicKeyInfo;
import org.bouncycastle.jce.provider.BouncyCastleProvider;
import org.bouncycastle.openssl.jcajce.JcaPEMKeyConverter;
import org.bouncycastle.openssl.jcajce.JcaPEMWriter;
import org.bouncycastle.util.io.pem.PemWriter;
import org.codehaus.plexus.util.StringOutputStream;
import org.junit.jupiter.api.Test;

public class KeyTest {
    final static String CERTIFICATE = "CERTIFICATE";
    final static String PRIVATE_KEY = "PRIVATE KEY";

    @Test
    void test() throws Exception {
        String config = """
                swirld, 298
                app, HederaNode.jar
                address, 0, node0, node0, 1, 0.0.0.0, 50111, 0.0.0.0, 50111, 0.0.3
                address, 1, node1, node1, 1, 0.0.0.0, 50111, 0.0.0.0, 50111, 0.0.4
                address, 2, node2, node2, 1, 0.0.0.0, 50111, 0.0.0.0, 50111, 0.0.5
                nextNodeId, 3
                """;
        Map<NodeId, KeysAndCerts> map = CryptoStatic.generateKeysAndCerts(AddressBookUtils.parseAddressBookText(config));
        StringWriter writer = new StringWriter();
        PrivateKeyInfo.getInstance(map.get(new NodeId(0)).sigKeyPair().getPrivate().getEncoded());
        SubjectPublicKeyInfo.getInstance(map.get(new NodeId(0)).sigKeyPair().getPublic().getEncoded());
        new JcaPEMWriter(writer).writeObject(map.get(new NodeId(0)).sigKeyPair().getPrivate());
        write("/tmp/apriv", PRIVATE_KEY, map.get(new NodeId(0)).agrKeyPair().getPrivate().getEncoded());
        write("/tmp/apub", CERTIFICATE, map.get(new NodeId(0)).agrCert().getEncoded());
        write("/tmp/spriv", PRIVATE_KEY, map.get(new NodeId(0)).sigKeyPair().getPrivate().getEncoded());
        write("/tmp/spub", CERTIFICATE, map.get(new NodeId(0)).sigCert().getEncoded());
        int i = 1;
    }

    void write(String filename, String label, byte[] data) throws Exception {
        try (PrintStream out = new PrintStream(new FileOutputStream(filename), true, StandardCharsets.UTF_8)) {
            out.println("-----BEGIN " + label + "-----");
//            byte[] enc = Base64.getEncoder().encode(data);
//            out.println(new String(enc));
//            ByteBuffer in = ByteBuffer.wrap(data);
            ByteBuffer bb = Base64.getEncoder().encode(ByteBuffer.wrap(data));
            byte[] tmp = new byte[64];
            while (bb.hasRemaining()) {
                int length = Math.min(64, bb.remaining());
                bb.get(tmp, 0, length);
                out.println(new String(tmp, 0, length, StandardCharsets.ISO_8859_1));
            }
//            ByteArrayOutputStream byteArrayOutputStream = new ByteArrayOutputStream();
//            PrintStream ps = new PrintStream(Base64.getEncoder().wrap(byteArrayOutputStream), true, StandardCharsets.UTF_8);
//            ps.write(data);
//            ps.close();
//            out.write(byteArrayOutputStream.toByteArray());
            out.println("-----END " + label + "-----");
        }
    }

    @Test
    void foo() throws Exception {
        Security.addProvider(new BouncyCastleProvider());

        final KeyPairGenerator sigKeyGen;
//        final KeyPairGenerator agrKeyGen;

        final SecureRandom secureRandom = SecureRandom.getInstanceStrong();

        sigKeyGen = KeyPairGenerator.getInstance("RSA", "BC");
//        agrKeyGen = KeyPairGenerator.getInstance("EC", "SunEC");

        sigKeyGen.initialize(CryptoConstants.SIG_KEY_SIZE_BITS, new SecureRandom());
//        agrKeyGen.initialize(CryptoConstants.AGR_KEY_SIZE_BITS, new SecureRandom());

        final KeyPair sigKeyPair = sigKeyGen.generateKeyPair();
//        final KeyPair agrKeyPair = agrKeyGen.generateKeyPair();

        final String dn = "CN=node1";

        // create the 2 certs (java.security.cert.Certificate)
        // both are signed by sigKeyPair, so sigCert is self-signed
//        KeyPair keyPair = KeysAndCerts.generateAgreementKeyPair();
        final X509Certificate cert =
                CryptoStatic.generateCertificate(dn, sigKeyPair, dn, sigKeyPair, SecureRandom.getInstanceStrong());

        write("/tmp/cert.crt", CERTIFICATE, cert.getEncoded());
        write("/tmp/cert.key", PRIVATE_KEY, sigKeyPair.getPrivate().getEncoded());
        int i = 1;
    }
}
