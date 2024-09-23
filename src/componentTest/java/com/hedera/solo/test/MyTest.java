package com.hedera.solo.test;

import java.util.HexFormat;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class MyTest {
    Logger log = LoggerFactory.getLogger(MyTest.class);
    final long msb = 1L << 3;
    final long half = msb >>> 1;
    final long mask = half | half - 1;

    //    static final long half = Long.MIN_VALUE >>> 1;
    int compare(long a, long b) {
        long diff = a - b & mask;
        return (diff | half) == half ? 0 : diff < half ? 1 : -1;
    }
    @Test
    public void test() {
//        assertEquals(hf, half);
//        assertEquals(mask, ~Long.MIN_VALUE);
        HexFormat hex = HexFormat.of();
        for (byte i = 0; i <= 0x7; ++i) {
            for (byte j = 0; j <= 0x7; ++j) {
                log.info("compare({}, {}): {}", hex.toHexDigits(i), hex.toHexDigits(j), compare(i, j));
            }
        }
    }
}
