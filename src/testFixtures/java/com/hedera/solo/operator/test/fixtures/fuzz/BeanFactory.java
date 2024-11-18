// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.fuzz;

import com.fasterxml.jackson.core.type.TypeReference;
import java.lang.invoke.MethodHandles;
import java.lang.reflect.Array;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.SequencedMap;
import java.util.Set;
import java.util.function.BiFunction;
import java.util.function.Predicate;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Create pseudo random data for testing. */
public class BeanFactory {
    private static final Logger log =
            LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private static final List<String> defaultCharset =
            IntStream.range(' ', '~' + 1).mapToObj(Character::toString).collect(Collectors.toList());
    private static final Map<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> defaultMap = defaultMap();
    protected List<String> charset = defaultCharset;
    protected LinkedHashMap<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> map = new LinkedHashMap<>();
    protected Random random = new Random();
    protected int arrayBound;
    protected int listBound;
    protected int mapBound;
    protected int setBound;
    protected int stringBound;
    protected double nullProbability;

    /** Default constructor with random seed. */
    public BeanFactory() {
        this(null);
    }

    /** Construct with PRNG seed if provided otherwise choose random seed. */
    public BeanFactory(final Long prngSeed) {
        long seed = prngSeed != null ? prngSeed : random.nextLong();
        random.setSeed(seed);
        log.info("seed: 0x{}L", HexFormat.of().toHexDigits(seed));
        maxArray(10).maxList(10).maxMap(10).maxSet(10).maxString(10).nullProbability(0.1);
    }

    /** Returns an instance of the given class with pseudo random values. */
    public <T> T nonNull(final Class<T> aClass) {
        return aClass.cast(nonNull((Type) aClass));
    }

    /** Returns stream of values which are instances of the given class with pseudo random values. */
    public <T> Stream<T> nonNulls(final Class<T> aClass) {
        return Stream.generate(() -> nonNull(aClass));
    }

    /** Returns either an instance of the given class with pseudo random values or null. */
    public <T> T random(final Class<T> aClass) {
        return randomNull(() -> nonNull(aClass));
    }

    /** Returns stream of values which are either instances of the given class with pseudo random values or null. */
    public <T> Stream<T> randoms(final Class<T> aClass) {
        return Stream.generate(() -> random(aClass));
    }

    /** Returns an instance of the given type with pseudo random values. */
    @SuppressWarnings("unchecked")
    public <T> T nonNull(final TypeReference<T> typeReference) {
        return (T) nonNull(typeReference.getType());
    }

    /** Returns stream of values which are instances of the given type with pseudo random values. */
    public <T> Stream<T> nonNulls(final TypeReference<T> typeReference) {
        return Stream.generate(() -> nonNull(typeReference));
    }

    /** Returns either an instance of the given type with pseudo random values or null. */
    public <T> T random(final TypeReference<T> typeReference) {
        return randomNull(() -> nonNull(typeReference));
    }

    /** Returns stream of values which are either instances of the given type with pseudo random values or null. */
    public <T> Stream<T> randoms(final TypeReference<T> typeReference) {
        return Stream.generate(() -> random(typeReference));
    }

    /**
     * Set upper bound on generated array size. Upper bound on total memory usage is exponential in the number of nested
     * "collections".
     */
    public BeanFactory maxArray(final int max) {
        arrayBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code List} size. Upper bound on total memory usage is exponential in the number of
     * nested "collections".
     */
    public BeanFactory maxList(final int max) {
        listBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code Map} size. Upper bound on total memory usage is exponential in the number of
     * nested "collections".
     */
    public BeanFactory maxMap(final int max) {
        mapBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code Set} size. Upper bound on total memory usage is exponential in the number of
     * nested "collections".
     */
    public BeanFactory maxSet(final int max) {
        setBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code String} size in terms of number of substrings chosen from {@code charset}.
     */
    public BeanFactory maxString(final int max) {
        stringBound = max + 1;
        return this;
    }

    /** Set the probability of generating a null value. */
    public BeanFactory nullProbability(final double probability) {
        nullProbability = probability;
        return this;
    }

    /** Set the character set used for generated strings. */
    public BeanFactory charset(final Collection<String> charset) {
        this.charset = new ArrayList<>(charset);
        return this;
    }

    /** Customize how objects are generated. */
    public BeanFactory suppliers(final SequencedMap<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> map) {
        this.map = new LinkedHashMap<>(map);
        return this;
    }

    public static Predicate<Type> isClass(final Class<?> aClass) {
        return type -> type instanceof Class<?> cls && cls.isAssignableFrom(aClass);
    }

    public static Predicate<Type> isClass(final Predicate<Class<?>> predicate) {
        return type -> type instanceof Class<?> aClass && predicate.test(aClass);
    }

    public static Predicate<Type> isParameterizedType(final Class<?> aClass) {
        return type -> type instanceof ParameterizedType parameterizedType
                && parameterizedType.getRawType() instanceof Class<?> rawClass
                && rawClass.isAssignableFrom(aClass);
    }

    protected Object nonNull(final Type type) {
        return Stream.concat(map.entrySet().stream(), defaultMap.entrySet().stream())
                .filter(e -> e.getKey().test(type))
                .findFirst()
                .map(Map.Entry::getValue)
                .orElseThrow(() -> new IllegalArgumentException("Unsupported type: " + type))
                .apply(this, type);
    }

    protected Object random(final Type type) {
        return randomNull(() -> nonNull(type));
    }

    protected <T> T randomNull(final Supplier<T> supplier) {
        return 0 != nullProbability && random.nextDouble() < nullProbability ? null : supplier.get();
    }

    protected String randomString(final Type type) {
        StringBuilder stringBuilder = new StringBuilder();
        for (int i = 0, limit = random.nextInt(stringBound); i < limit; i++) {
            stringBuilder.append(charset.get(random.nextInt(charset.size())));
        }
        return stringBuilder.toString();
    }

    protected Long randomLong(final Type type) {
        return random.nextLong();
    }

    protected Double randomDouble(final Type type) {
        return Double.longBitsToDouble(random.nextLong());
    }

    protected Integer randomInteger(final Type type) {
        return random.nextInt();
    }

    protected Float randomFloat(final Type type) {
        return Float.intBitsToFloat(random.nextInt());
    }

    protected Short randomShort(final Type type) {
        return (short) random.nextInt();
    }

    protected Character randomCharacter(final Type type) {
        return (char) random.nextInt();
    }

    protected Byte randomByte(final Type type) {
        return (byte) random.nextInt();
    }

    protected Boolean randomBoolean(final Type type) {
        return random.nextBoolean();
    }

    protected Object randomArray(final Type type) {
        int size = random.nextInt(arrayBound);
        Object array = Array.newInstance(((Class<?>) type).getComponentType(), size);
        IntStream.range(0, size).forEach(i -> Array.set(array, i, random(((Class<?>) type).getComponentType())));
        return array;
    }

    protected Object randomEnum(final Type type) {
        Class<?> enumClass = (Class<?>) type;
        return enumClass.getEnumConstants()[random.nextInt(enumClass.getEnumConstants().length)];
    }

    protected List<?> randomList(final Type type) {
        return randomStream(type).limit(random.nextInt(listBound)).toList();
    }

    protected Map<?, ?> randomMap(final Type type) {
        Type[] types = ((ParameterizedType) type).getActualTypeArguments();
        return IntStream.range(0, random.nextInt(mapBound))
                .collect(LinkedHashMap::new, (m, $) -> m.put(random(types[0]), random(types[1])), Map::putAll);
    }

    protected Set<?> randomSet(final Type type) {
        return randomStream(type).limit(random.nextInt(setBound)).collect(Collectors.toSet());
    }

    protected Stream<?> randomStream(final Type type) {
        return Stream.generate(() -> random(((ParameterizedType) type).getActualTypeArguments()[0]));
    }

    protected Object randomBean(final Type type) {
        try {
            Class<?> aClass = (Class<?>) type;
            Object object = aClass.getDeclaredConstructor().newInstance();
            List<Method> methods = Arrays.stream(aClass.getDeclaredMethods())
                    .filter(method -> 0 == (method.getModifiers() & Modifier.STATIC)
                            && method.canAccess(object)
                            && method.getName().startsWith("set"))
                    .sorted(Comparator.comparing(Method::getParameterCount)
                            .reversed()
                            .thenComparing(Method::toString))
                    .toList();
            for (Method method : methods) {
                method.invoke(
                        object,
                        Arrays.stream(method.getGenericParameterTypes())
                                .map(this::random)
                                .toArray());
            }
            return object;
        } catch (ReflectiveOperationException exception) {
            throw new RuntimeException(exception);
        }
    }

    private static Map<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> defaultMap() {
        Map<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> defaultMap = new LinkedHashMap<>();
        defaultMap.put(isClass(String.class), BeanFactory::randomString);
        defaultMap.put(isClass(Long.class), BeanFactory::randomLong);
        defaultMap.put(isClass(Double.class), BeanFactory::randomDouble);
        defaultMap.put(isClass(Integer.class), BeanFactory::randomInteger);
        defaultMap.put(isClass(Float.class), BeanFactory::randomFloat);
        defaultMap.put(isClass(Short.class), BeanFactory::randomShort);
        defaultMap.put(isClass(Character.class), BeanFactory::randomCharacter);
        defaultMap.put(isClass(Byte.class), BeanFactory::randomByte);
        defaultMap.put(isClass(Boolean.class), BeanFactory::randomBoolean);
        defaultMap.put(isClass(Class::isArray), BeanFactory::randomArray);
        defaultMap.put(isClass(Class::isEnum), BeanFactory::randomEnum);
        defaultMap.put(t -> t instanceof Class<?>, BeanFactory::randomBean);
        defaultMap.put(isParameterizedType(List.class), BeanFactory::randomList);
        defaultMap.put(isParameterizedType(Map.class), BeanFactory::randomMap);
        defaultMap.put(isParameterizedType(Set.class), BeanFactory::randomSet);
        defaultMap.put(isParameterizedType(Stream.class), BeanFactory::randomStream);
        return defaultMap;
    }
}
