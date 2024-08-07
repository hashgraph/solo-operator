import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class My1Test {
//    private final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass().getName());
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    @Test
    public void test() {
        log.debug("Hello World!");
        log.info("Hello World!");
        log.error("Hello World!");
    }
}
