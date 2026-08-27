cmake_minimum_required(VERSION 3.21)

project(dipole_realtime_delivery LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

find_package(PkgConfig REQUIRED)
find_package(nlohmann_json 3.11 REQUIRED)
pkg_check_modules(PROTOBUF REQUIRED IMPORTED_TARGET "protobuf>=3.21")
pkg_check_modules(HIREDIS REQUIRED IMPORTED_TARGET "hiredis>=1.2")
find_program(PROTOC_EXECUTABLE protoc REQUIRED NO_CMAKE_FIND_ROOT_PATH)

set(DIPOLE_RDKAFKA_ROOT "" CACHE PATH "Optional extracted librdkafka package root")
if(DIPOLE_RDKAFKA_ROOT)
  find_path(RDKAFKA_INCLUDE_DIR librdkafka/rdkafka.h
    PATHS "${DIPOLE_RDKAFKA_ROOT}/usr/include" NO_DEFAULT_PATH REQUIRED)
  find_library(RDKAFKA_LIBRARY rdkafka
    PATHS "${DIPOLE_RDKAFKA_ROOT}/usr/lib/x86_64-linux-gnu" NO_DEFAULT_PATH REQUIRED)
  add_library(RDKAFKA::rdkafka UNKNOWN IMPORTED)
  set_target_properties(RDKAFKA::rdkafka PROPERTIES
    IMPORTED_LOCATION "${RDKAFKA_LIBRARY}"
    INTERFACE_INCLUDE_DIRECTORIES "${RDKAFKA_INCLUDE_DIR}"
  )
else()
  pkg_check_modules(RDKAFKA REQUIRED IMPORTED_TARGET "rdkafka>=2.3")
  add_library(RDKAFKA::rdkafka ALIAS PkgConfig::RDKAFKA)
endif()

set(DIPOLE_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/..")
set(PROTO_ROOT "${DIPOLE_ROOT}/api/proto")
set(DELIVERY_PROTO "${PROTO_ROOT}/dipole/delivery/v1/delivery.proto")
set(GENERATED_ROOT "${CMAKE_CURRENT_BINARY_DIR}/generated")
set(DELIVERY_PROTO_CC "${GENERATED_ROOT}/dipole/delivery/v1/delivery.pb.cc")
set(DELIVERY_PROTO_H "${GENERATED_ROOT}/dipole/delivery/v1/delivery.pb.h")

add_custom_command(
  OUTPUT "${DELIVERY_PROTO_CC}" "${DELIVERY_PROTO_H}"
  COMMAND "${CMAKE_COMMAND}" -E make_directory "${GENERATED_ROOT}"
  COMMAND "${PROTOC_EXECUTABLE}"
    "--cpp_out=${GENERATED_ROOT}"
    "--proto_path=${PROTO_ROOT}"
    "${DELIVERY_PROTO}"
  DEPENDS "${DELIVERY_PROTO}"
  VERBATIM
)

add_library(delivery_contract
  src/contract_validator.cpp
  src/event_projection.cpp
  src/health_server.cpp
  src/hiredis_presence_reader.cpp
  src/librdkafka_consumer.cpp
  src/presence_projection.cpp
  src/shadow_evidence.cpp
  src/shadow_runner.cpp
  src/shadow_runtime.cpp
  "${DELIVERY_PROTO_CC}"
)
target_include_directories(delivery_contract PUBLIC
  "${CMAKE_CURRENT_SOURCE_DIR}/src"
  "${GENERATED_ROOT}"
)
target_link_libraries(delivery_contract PUBLIC
  PkgConfig::PROTOBUF
  PkgConfig::HIREDIS
  RDKAFKA::rdkafka
  nlohmann_json::nlohmann_json
)
target_compile_options(delivery_contract PRIVATE -Wall -Wextra -Wpedantic -Werror)

add_executable(dipole-realtime-delivery src/main.cpp)
target_link_libraries(dipole-realtime-delivery PRIVATE delivery_contract)
target_compile_options(dipole-realtime-delivery PRIVATE -Wall -Wextra -Wpedantic -Werror)

include(CTest)
if(BUILD_TESTING)
  add_executable(delivery_contract_test tests/contract_test.cpp)
  target_link_libraries(delivery_contract_test PRIVATE delivery_contract)
  target_compile_options(delivery_contract_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  target_compile_definitions(delivery_contract_test PRIVATE
    DIPOLE_DELIVERY_TESTDATA_DIR="${PROTO_ROOT}/dipole/delivery/v1/testdata"
  )
  add_test(NAME delivery_contract_test COMMAND delivery_contract_test)

  add_executable(delivery_projection_test tests/event_projection_test.cpp)
  target_link_libraries(delivery_projection_test PRIVATE delivery_contract)
  target_compile_options(delivery_projection_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_projection_test COMMAND delivery_projection_test)

  add_executable(delivery_shadow_runner_test tests/shadow_runner_test.cpp)
  target_link_libraries(delivery_shadow_runner_test PRIVATE delivery_contract)
  target_compile_options(delivery_shadow_runner_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_shadow_runner_test COMMAND delivery_shadow_runner_test)

  add_executable(delivery_librdkafka_consumer_test tests/librdkafka_consumer_test.cpp)
  target_link_libraries(delivery_librdkafka_consumer_test PRIVATE delivery_contract)
  target_compile_options(delivery_librdkafka_consumer_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_librdkafka_consumer_test COMMAND delivery_librdkafka_consumer_test)

  add_executable(delivery_shadow_evidence_test tests/shadow_evidence_test.cpp)
  target_link_libraries(delivery_shadow_evidence_test PRIVATE delivery_contract)
  target_compile_options(delivery_shadow_evidence_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_shadow_evidence_test COMMAND delivery_shadow_evidence_test)

  add_executable(delivery_presence_projection_test tests/presence_projection_test.cpp)
  target_link_libraries(delivery_presence_projection_test PRIVATE delivery_contract)
  target_compile_options(delivery_presence_projection_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_presence_projection_test COMMAND delivery_presence_projection_test)

  add_executable(delivery_hiredis_presence_reader_test tests/hiredis_presence_reader_test.cpp)
  target_link_libraries(delivery_hiredis_presence_reader_test PRIVATE delivery_contract)
  target_compile_options(delivery_hiredis_presence_reader_test PRIVATE -Wall -Wextra -Wpedantic -Werror)
  add_test(NAME delivery_hiredis_presence_reader_test COMMAND delivery_hiredis_presence_reader_test)
  add_test(
    NAME delivery_contract_cli
    COMMAND dipole-realtime-delivery validate "${PROTO_ROOT}/dipole/delivery/v1/testdata"
  )
  add_test(
    NAME delivery_health
    COMMAND "${CMAKE_CURRENT_SOURCE_DIR}/tests/health_test.sh"
      "$<TARGET_FILE:dipole-realtime-delivery>"
      "${PROTO_ROOT}/dipole/delivery/v1/testdata"
  )
  set_tests_properties(delivery_health PROPERTIES TIMEOUT 15)
  add_test(
    NAME delivery_shadow_health
    COMMAND "${CMAKE_CURRENT_SOURCE_DIR}/tests/shadow_health_test.sh"
      "$<TARGET_FILE:dipole-realtime-delivery>"
      "${PROTO_ROOT}/dipole/delivery/v1/testdata"
  )
  set_tests_properties(delivery_shadow_health PROPERTIES TIMEOUT 15)
endif()
