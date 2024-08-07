import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.junit.jupiter.Testcontainers;

public class MyTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    @Test
    public void test() {
        log.info("Hello World!");
    }
}
