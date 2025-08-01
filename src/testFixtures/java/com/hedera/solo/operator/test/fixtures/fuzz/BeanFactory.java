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
    protected SequencedMap<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> map = new LinkedHashMap<>();
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

    /**
     * Construct with PRNG seed if provided otherwise choose random seed.
     *
     * @param prngSeed the seed for the pseudo random number generator, or null to use a random seed
     */
    public BeanFactory(final Long prngSeed) {
        long seed = prngSeed != null ? prngSeed : random.nextLong();
        random.setSeed(seed);
        log.info("seed: 0x{}L", HexFormat.of().toHexDigits(seed));
        maxArray(10).maxList(10).maxMap(10).maxSet(10).maxString(10).nullProbability(0.1);
    }

    /**
     * Access PRNG.
     *
     * @return the instance of the pseudo random number generator used by this BeanFactory
     */
    public Random random() {
        return random;
    }

    /**
     * Returns an instance of the given class with pseudo random values.
     *
     * @param aClass the class of the type to be generated
     * @param <T> the type of the object to be generated
     * @return an instance of the type with pseudo random values
     */
    public <T> T nonNull(final Class<T> aClass) {
        return aClass.cast(nonNull((Type) aClass));
    }

    /**
     * Returns stream of values which are instances of the given class with pseudo random values.
     *
     * @param aClass the class of the type to be generated
     * @param <T> the type of the object to be generated
     * @return stream of random values
     */
    public <T> Stream<T> nonNulls(final Class<T> aClass) {
        return Stream.generate(() -> nonNull(aClass));
    }

    /**
     * Returns either an instance of the given class with pseudo random values or null.
     *
     * @param aClass the class of the type to be generated
     * @param <T> the type of the object to be generated
     * @return an instance of the type with pseudo random values or null
     */
    public <T> T random(final Class<T> aClass) {
        return randomNull(() -> nonNull(aClass));
    }

    /**
     * Returns stream of values which are either instances of the given class with pseudo random values or null.
     *
     * @param aClass the class of the type to be generated
     * @param <T> the type of the object to be generated
     * @return stream of random values
     */
    public <T> Stream<T> randoms(final Class<T> aClass) {
        return Stream.generate(() -> random(aClass));
    }

    /**
     * Returns an instance of the given type with pseudo random values.
     *
     * @param typeReference a TypeReference for the type to be generated
     * @param <T> the type of the object to be generated
     * @return an instance of the type with pseudo random values
     */
    @SuppressWarnings("unchecked")
    public <T> T nonNull(final TypeReference<T> typeReference) {
        return (T) nonNull(typeReference.getType());
    }

    /**
     * Returns stream of values which are instances of the given type with pseudo random values.
     *
     * @param typeReference a TypeReference for the type to be generated
     * @param <T> the type of the object to be generated
     * @return stream of random values
     */
    public <T> Stream<T> nonNulls(final TypeReference<T> typeReference) {
        return Stream.generate(() -> nonNull(typeReference));
    }

    /**
     * Returns either an instance of the given type with pseudo random values or null.
     *
     * @param typeReference a TypeReference for the type to be generated
     * @param <T> the type of the object to be generated
     * @return an instance of the type with pseudo random values or null
     */
    public <T> T random(final TypeReference<T> typeReference) {
        return randomNull(() -> nonNull(typeReference));
    }

    /**
     * Returns stream of values which are either instances of the given type with pseudo random values or null.
     *
     * @param typeReference a TypeReference for the type to be generated
     * @param <T> the type of the object to be generated
     * @return stream of random values
     */
    public <T> Stream<T> randoms(final TypeReference<T> typeReference) {
        return Stream.generate(() -> random(typeReference));
    }

    /**
     * Set upper bound on generated array size.
     *
     * <p>Note: Upper bound on total memory usage is exponential in the number of nested "collections".
     *
     * @param max the maximum number of elements in the generated array
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory maxArray(final int max) {
        arrayBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code List} size.
     *
     * <p>Note: Upper bound on total memory usage is exponential in the number of nested "collections".
     *
     * @param max the maximum number of elements in the generated list
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory maxList(final int max) {
        listBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code Map} size.
     *
     * <p>Note: Upper bound on total memory usage is exponential in the number of nested "collections".
     *
     * @param max the maximum number of entries in the generated map
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory maxMap(final int max) {
        mapBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code Set} size.
     *
     * <p>Note: Upper bound on total memory usage is exponential in the number of nested "collections".
     *
     * @param max the maximum number of elements in the generated set
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory maxSet(final int max) {
        setBound = max + 1;
        return this;
    }

    /**
     * Set upper bound on generated {@code String} size in terms of number of substrings chosen from {@code charset}.
     *
     * @param max the maximum number of characters in the generated string
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory maxString(final int max) {
        stringBound = max + 1;
        return this;
    }

    /**
     * Set the probability of generating a null value.
     *
     * @param probability a double between 0.0 and 1.0, where 0.0 means never null and 1.0 means always null
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory nullProbability(final double probability) {
        nullProbability = probability;
        return this;
    }

    /**
     * Set the character set used for generated strings.
     *
     * @param chars a collection of characters to be used in generated strings
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory charset(final Collection<String> chars) {
        charset = new ArrayList<>(chars);
        return this;
    }

    /**
     * Customize how objects are generated.
     *
     * @param aClass the class of the type to be generated
     * @param supplier a function that takes the BeanFactory and Type, and returns an instance of the type
     * @param <T> the type of the object to be generated
     * @return this BeanFactory instance for method chaining
     */
    public <T> BeanFactory supplier(final Class<T> aClass, final BiFunction<BeanFactory, Type, Object> supplier) {
        map.put(aClass::equals, supplier);
        return this;
    }

    /**
     * Customize how objects are generated.
     *
     * @param typeReference a TypeReference for the type to be generated
     * @param supplier a function that takes the BeanFactory and Type, and returns an instance of the type
     * @param <T> the type of the object to be generated
     * @return this BeanFactory instance for method chaining
     */
    public <T> BeanFactory supplier(
            final TypeReference<T> typeReference, final BiFunction<BeanFactory, Type, Object> supplier) {
        map.put(typeReference.getType()::equals, supplier);
        return this;
    }

    /**
     * Customize how objects are generated.
     *
     * @param suppliers a map of predicates to suppliers
     * @return this BeanFactory instance for method chaining
     */
    public BeanFactory suppliers(final SequencedMap<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> suppliers) {
        map = new LinkedHashMap<>(suppliers);
        return this;
    }

    /**
     * Predicate that checks if the given type is a class that matches the specified class.
     *
     * @param aClass the class to compare against
     * @return a predicate for checking type assignability
     */
    protected static Predicate<Type> isClass(final Class<?> aClass) {
        return type -> type instanceof Class<?> cls && cls.isAssignableFrom(aClass);
    }

    /**
     * Predicate that checks if the given type is an instance of a class that matches the specified predicate.
     *
     * @param predicate the predicate to apply to the class
     * @return a predicate for checking class match with a condition
     */
    protected static Predicate<Type> isClass(final Predicate<Class<?>> predicate) {
        return type -> type instanceof Class<?> aClass && predicate.test(aClass);
    }

    /**
     * Predicate that checks if the given type is a parameterized type whose raw type is assignable from the specified
     * class.
     *
     * @param aClass the class to compare against
     * @return a predicate for checking parameterized type assignability
     */
    protected static Predicate<Type> isParameterizedType(final Class<?> aClass) {
        return type -> type instanceof ParameterizedType parameterizedType
                && parameterizedType.getRawType() instanceof Class<?> rawClass
                && rawClass.isAssignableFrom(aClass);
    }

    /**
     * Returns a non-null object of the specified type, using the map of suppliers.
     *
     * @param type the type of the object to generate
     * @return a non-null object of the specified type
     * @throws IllegalArgumentException if no supplier is found for the specified type
     */
    protected Object nonNull(final Type type) {
        return Stream.concat(map.entrySet().stream(), defaultMap.entrySet().stream())
                .filter(e -> e.getKey().test(type))
                .findFirst()
                .map(Map.Entry::getValue)
                .orElseThrow(() -> new IllegalArgumentException("Unsupported type: " + type))
                .apply(this, type);
    }

    /**
     * Returns a randomly generated object of the specified type, potentially null based on the null probability.
     *
     * @param type the type of the object to generate
     * @return a random object or null if the random decision is made to return null
     */
    protected Object random(final Type type) {
        return randomNull(() -> nonNull(type));
    }

    /**
     * Returns either a randomly generated object from the supplier or null based on the null probability.
     *
     * @param supplier the supplier of the object to generate
     * @param <T> the type of the object to generate
     * @return a random object or null if the random decision is made to return null
     */
    protected <T> T randomNull(final Supplier<T> supplier) {
        return 0 != nullProbability && random.nextDouble() < nullProbability ? null : supplier.get();
    }

    /**
     * Returns a randomly generated string of characters from the charset.
     *
     * @param type the type of the string to generate (should be String.class)
     * @return a random string
     */
    protected String randomString(final Type type) {
        StringBuilder stringBuilder = new StringBuilder();
        for (int i = 0, limit = random.nextInt(stringBound); i < limit; i++) {
            stringBuilder.append(charset.get(random.nextInt(charset.size())));
        }
        return stringBuilder.toString();
    }

    /**
     * Returns a randomly generated long value.
     *
     * @param type the type of the long value to generate (should be Long.class)
     * @return a random long value
     */
    protected Long randomLong(final Type type) {
        return random.nextLong();
    }

    /**
     * Returns a randomly generated double value.
     *
     * @param type the type of the double value to generate (should be Double.class)
     * @return a random double value
     */
    protected Double randomDouble(final Type type) {
        return Double.longBitsToDouble(random.nextLong());
    }

    /**
     * Returns a randomly generated integer value.
     *
     * @param type the type of the integer value to generate (should be Integer.class)
     * @return a random integer value
     */
    protected Integer randomInteger(final Type type) {
        return random.nextInt();
    }

    /**
     * Returns a randomly generated float value.
     *
     * @param type the type of the float value to generate (should be Float.class)
     * @return a random float value
     */
    protected Float randomFloat(final Type type) {
        return Float.intBitsToFloat(random.nextInt());
    }

    /**
     * Returns a randomly generated short value.
     *
     * @param type the type of the short value to generate (should be Short.class)
     * @return a random short value
     */
    protected Short randomShort(final Type type) {
        return (short) random.nextInt();
    }

    /**
     * Returns a randomly generated character value.
     *
     * @param type the type of the character value to generate (should be Character.class)
     * @return a random character value
     */
    protected Character randomCharacter(final Type type) {
        return (char) random.nextInt();
    }

    /**
     * Returns a randomly generated byte value.
     *
     * @param type the type of the byte value to generate (should be Byte.class)
     * @return a random byte value
     */
    protected Byte randomByte(final Type type) {
        return (byte) random.nextInt();
    }

    /**
     * Returns a randomly generated boolean value.
     *
     * @param type the type of the boolean value to generate (should be Boolean.class)
     * @return a random boolean value
     */
    protected Boolean randomBoolean(final Type type) {
        return random.nextBoolean();
    }

    /**
     * Returns a randomly generated array of objects of the specified component type.
     *
     * @param type the type of the array to generate (should be an array class)
     * @return a random array
     */
    protected Object randomArray(final Type type) {
        int size = random.nextInt(arrayBound);
        Object array = Array.newInstance(((Class<?>) type).getComponentType(), size);
        IntStream.range(0, size).forEach(i -> Array.set(array, i, random(((Class<?>) type).getComponentType())));
        return array;
    }

    /**
     * Returns a randomly selected enum constant from the specified enum class.
     *
     * @param type the type of the enum class to generate (should be an enum class)
     * @return a random enum constant
     */
    protected Object randomEnum(final Type type) {
        Class<?> enumClass = (Class<?>) type;
        return enumClass.getEnumConstants()[random.nextInt(enumClass.getEnumConstants().length)];
    }

    /**
     * Returns a randomly generated list of objects of the specified parameterized type.
     *
     * @param type the type of the list to generate (should be a ParameterizedType of List)
     * @return a random list
     */
    protected List<?> randomList(final Type type) {
        return randomStream(type).limit(random.nextInt(listBound)).toList();
    }

    /**
     * Returns a randomly generated map of objects of the specified parameterized type.
     *
     * @param type the type of the map to generate (should be a ParameterizedType of Map)
     * @return a random map
     */
    protected Map<?, ?> randomMap(final Type type) {
        Type[] types = ((ParameterizedType) type).getActualTypeArguments();
        return IntStream.range(0, random.nextInt(mapBound))
                .collect(LinkedHashMap::new, (m, $) -> m.put(random(types[0]), random(types[1])), Map::putAll);
    }

    /**
     * Returns a randomly generated set of objects of the specified parameterized type.
     *
     * @param type the type of the set to generate (should be a ParameterizedType of Set)
     * @return a random set
     */
    protected Set<?> randomSet(final Type type) {
        return randomStream(type).limit(random.nextInt(setBound)).collect(Collectors.toSet());
    }

    /**
     * Returns a stream of randomly generated objects of the specified parameterized type.
     *
     * @param type the type of the stream to generate (should be a ParameterizedType of Stream)
     * @return a random stream
     */
    protected Stream<?> randomStream(final Type type) {
        return Stream.generate(() -> random(((ParameterizedType) type).getActualTypeArguments()[0]));
    }

    /**
     * Returns a randomly generated bean of the specified class, with its fields set to random values.
     *
     * @param type the type of the bean to generate (should be a Class)
     * @return a random bean
     * @throws RuntimeException if an error occurs during bean instantiation or method invocation
     */
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

    /**
     * Returns a map of predicates to suppliers for default random object generation.
     *
     * @return a map of predicates to suppliers
     */
    private static Map<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> defaultMap() {
        Map<Predicate<Type>, BiFunction<BeanFactory, Type, Object>> suppliers = new LinkedHashMap<>();
        suppliers.put(isClass(String.class), BeanFactory::randomString);
        suppliers.put(isClass(Long.class), BeanFactory::randomLong);
        suppliers.put(isClass(Double.class), BeanFactory::randomDouble);
        suppliers.put(isClass(Integer.class), BeanFactory::randomInteger);
        suppliers.put(isClass(Float.class), BeanFactory::randomFloat);
        suppliers.put(isClass(Short.class), BeanFactory::randomShort);
        suppliers.put(isClass(Character.class), BeanFactory::randomCharacter);
        suppliers.put(isClass(Byte.class), BeanFactory::randomByte);
        suppliers.put(isClass(Boolean.class), BeanFactory::randomBoolean);
        suppliers.put(isClass(Class::isArray), BeanFactory::randomArray);
        suppliers.put(isClass(Class::isEnum), BeanFactory::randomEnum);
        suppliers.put(t -> t instanceof Class<?>, BeanFactory::randomBean);
        suppliers.put(isParameterizedType(List.class), BeanFactory::randomList);
        suppliers.put(isParameterizedType(Map.class), BeanFactory::randomMap);
        suppliers.put(isParameterizedType(Set.class), BeanFactory::randomSet);
        suppliers.put(isParameterizedType(Stream.class), BeanFactory::randomStream);
        return suppliers;
    }
}
