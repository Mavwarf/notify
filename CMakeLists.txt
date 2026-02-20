cmake_minimum_required(VERSION 3.16)
project(notify NONE) # No C/C++ compiler needed — this is a Go project

# --- Find Go ---
find_program(GO_EXECUTABLE go REQUIRED)
execute_process(
    COMMAND ${GO_EXECUTABLE} version
    OUTPUT_VARIABLE GO_VERSION_OUTPUT
    OUTPUT_STRIP_TRAILING_WHITESPACE
)
message(STATUS "Found Go: ${GO_EXECUTABLE} (${GO_VERSION_OUTPUT})")

set(SOURCE_DIR  "${CMAKE_CURRENT_SOURCE_DIR}")
set(OUTPUT_DIR  "${SOURCE_DIR}/output")

file(MAKE_DIRECTORY "${OUTPUT_DIR}")

# ===========================================================================
# notify
# ===========================================================================
set(NT_NAME "notify")

# --- Default build (native) ---
add_custom_target(build-notify
    COMMAND ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}${CMAKE_EXECUTABLE_SUFFIX}" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for the current platform"
)

add_custom_target(build ALL
    DEPENDS build-notify
    COMMENT "Building all tools for the current platform"
)

# --- Install ---
install(
    PROGRAMS "${OUTPUT_DIR}/${NT_NAME}${CMAKE_EXECUTABLE_SUFFIX}"
    DESTINATION bin
)

# ===========================================================================
# Cross-compilation
# ===========================================================================
add_custom_target(build-notify-linux-amd64
    COMMAND ${CMAKE_COMMAND} -E env GOOS=linux GOARCH=amd64
            ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}-linux-amd64" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for Linux (amd64)"
)

add_custom_target(build-notify-linux-arm64
    COMMAND ${CMAKE_COMMAND} -E env GOOS=linux GOARCH=arm64
            ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}-linux-arm64" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for Linux (arm64)"
)

add_custom_target(build-notify-windows-amd64
    COMMAND ${CMAKE_COMMAND} -E env GOOS=windows GOARCH=amd64
            ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}-windows-amd64.exe" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for Windows (amd64)"
)

add_custom_target(build-notify-darwin-amd64
    COMMAND ${CMAKE_COMMAND} -E env GOOS=darwin GOARCH=amd64
            ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}-darwin-amd64" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for macOS (amd64)"
)

add_custom_target(build-notify-darwin-arm64
    COMMAND ${CMAKE_COMMAND} -E env GOOS=darwin GOARCH=arm64
            ${GO_EXECUTABLE} build -o "${OUTPUT_DIR}/${NT_NAME}-darwin-arm64" ./cmd/notify
    WORKING_DIRECTORY "${SOURCE_DIR}"
    COMMENT "Building ${NT_NAME} for macOS (arm64 / Apple Silicon)"
)

add_custom_target(build-notify-all
    DEPENDS build-notify-linux-amd64 build-notify-linux-arm64
            build-notify-windows-amd64
            build-notify-darwin-amd64 build-notify-darwin-arm64
    COMMENT "Building ${NT_NAME} for all platforms"
)

add_custom_target(build-all
    DEPENDS build-notify-all
    COMMENT "Building all tools for all platforms"
)

# --- Clean ---
set_property(
    DIRECTORY APPEND PROPERTY ADDITIONAL_CLEAN_FILES
    "${OUTPUT_DIR}/${NT_NAME}${CMAKE_EXECUTABLE_SUFFIX}"
    "${OUTPUT_DIR}/${NT_NAME}-linux-amd64"
    "${OUTPUT_DIR}/${NT_NAME}-linux-arm64"
    "${OUTPUT_DIR}/${NT_NAME}-windows-amd64.exe"
    "${OUTPUT_DIR}/${NT_NAME}-darwin-amd64"
    "${OUTPUT_DIR}/${NT_NAME}-darwin-arm64"
)
