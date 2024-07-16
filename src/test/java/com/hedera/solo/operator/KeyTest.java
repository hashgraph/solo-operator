package com.hedera.solo.operator;

import com.swirlds.common.platform.NodeId;
import com.swirlds.platform.crypto.CryptoConstants;
import com.swirlds.platform.crypto.CryptoStatic;
import com.swirlds.platform.crypto.KeysAndCerts;
import com.swirlds.platform.system.address.AddressBookUtils;
import java.io.FileOutputStream;
import java.io.PrintStream;
import java.io.StringWriter;
import java.math.BigInteger;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.SecureRandom;
import java.security.Security;
import java.security.cert.X509Certificate;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Date;
import java.util.List;
import java.util.Map;
import javax.security.auth.x500.X500Principal;
import org.bouncycastle.asn1.DERSequence;
import org.bouncycastle.asn1.pkcs.PrivateKeyInfo;
import org.bouncycastle.asn1.x509.Extension;
import org.bouncycastle.asn1.x509.GeneralName;
import org.bouncycastle.asn1.x509.GeneralNames;
import org.bouncycastle.asn1.x509.SubjectPublicKeyInfo;
import org.bouncycastle.cert.X509v3CertificateBuilder;
import org.bouncycastle.cert.jcajce.JcaX509CertificateConverter;
import org.bouncycastle.cert.jcajce.JcaX509v3CertificateBuilder;
import org.bouncycastle.jce.provider.BouncyCastleProvider;
import org.bouncycastle.openssl.jcajce.JcaPEMWriter;
import org.bouncycastle.operator.jcajce.JcaContentSignerBuilder;
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
        try (PrintStream out = new PrintStream(new FileOutputStream(filename))) {
            out.println("-----BEGIN " + label + "-----");
//            byte[] enc = Base64.getEncoder().encode(data);
//            out.println(new String(enc));
//            ByteBuffer in = ByteBuffer.wrap(data);
            out.println(new String(Base64.getMimeEncoder(65, new byte[]{'\n'}).encode(data)));
//            ByteBuffer bb = Base64.getEncoder().encode(ByteBuffer.wrap(data));
//            byte[] tmp = new byte[64];
//            while (bb.hasRemaining()) {
//                int length = Math.min(64, bb.remaining());
//                bb.get(tmp, 0, length);
//                out.println(new String(tmp, 0, length, StandardCharsets.ISO_8859_1));
//            }
//            ByteArrayOutputStream byteArrayOutputStream = new ByteArrayOutputStream();
//            PrintStream ps = new PrintStream(Base64.getEncoder().wrap(byteArrayOutputStream), true, StandardCharsets.UTF_8);
//            ps.write(data);
//            ps.close();
//            out.write(byteArrayOutputStream.toByteArray());
            out.println("-----END " + label + "-----");
        }
    }

    void generate(String name) throws Exception {
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

        final String dn = "CN=" + name;//,OID.2.5.29.17=localhost\\,node0";

        // create the 2 certs (java.security.cert.Certificate)
        // both are signed by sigKeyPair, so sigCert is self-signed
//        KeyPair keyPair = KeysAndCerts.generateAgreementKeyPair();
        final X509Certificate cert =
                generateCertificate(dn, sigKeyPair, dn, sigKeyPair, SecureRandom.getInstanceStrong());

        write("/tmp/" + name + ".crt", CERTIFICATE, cert.getEncoded());
        write("/tmp/" + name + ".key", PRIVATE_KEY, sigKeyPair.getPrivate().getEncoded());
    }

    @Test
    void foo() throws Exception {
        generate("node0");
        generate("node1");
        generate("node2");
    }

    private static final int SERIAL_NUMBER_BITS = 64;

    public static X509Certificate generateCertificate(
            String distinguishedName,
            KeyPair pair,
            String caDistinguishedName,
            KeyPair caPair,
            SecureRandom secureRandom)
            throws Exception {
        X509v3CertificateBuilder v3CertBldr = new JcaX509v3CertificateBuilder(
                new X500Principal(caDistinguishedName), // issuer
                new BigInteger(SERIAL_NUMBER_BITS, secureRandom), // serial number
                Date.from(CryptoConstants.DEFAULT_VALID_FROM), // start time
                Date.from(CryptoConstants.DEFAULT_VALID_TO), // expiry time
                new X500Principal(distinguishedName), // subject
                pair.getPublic()); // subject public key

        List<GeneralName> altNames = new ArrayList<>();
        altNames.add(new GeneralName(GeneralName.iPAddress, "127.0.0.1"));
        GeneralNames subjectAltNames = GeneralNames.getInstance(new DERSequence(altNames.toArray(new GeneralName[]{})));
        v3CertBldr.addExtension(Extension.subjectAlternativeName, false, subjectAltNames);

        JcaContentSignerBuilder signerBuilder = new JcaContentSignerBuilder(CryptoConstants.SIG_TYPE2)
                .setProvider(BouncyCastleProvider.PROVIDER_NAME);
        return new JcaX509CertificateConverter()
                .setProvider(BouncyCastleProvider.PROVIDER_NAME)
                .getCertificate(v3CertBldr.build(signerBuilder.build(caPair.getPrivate())));
    }
}
