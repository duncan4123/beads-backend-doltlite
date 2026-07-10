const std = @import("std");

const Binary = struct {
    name: []const u8,
    package: []const u8,
};

const binaries = [_]Binary{
    .{ .name = "bd-backend-doltlite", .package = "./cmd/bd-backend-doltlite" },
    .{ .name = "gc-doltlite-fastpath", .package = "./cmd/gc-doltlite-fastpath" },
    .{ .name = "gc-doltlite", .package = "./cmd/gc-doltlite" },
    .{ .name = "doltlite-client", .package = "./cmd/doltlite-client" },
};

pub fn build(b: *std.Build) void {
    const doltlite_lib = b.option(
        []const u8,
        "doltlite-lib",
        "Existing directory containing doltlite.h and libdoltlite (otherwise the locked release is downloaded)",
    ) orelse "";
    const cache_root = b.option(
        []const u8,
        "native-cache",
        "Native dependency cache directory",
    ) orelse ".zig-cache/native";
    const output_root = b.option(
        []const u8,
        "output-dir",
        "Plugin output directory",
    ) orelse "zig-out";
    const go_exe = b.option([]const u8, "go", "Go executable") orelse "go";

    const dependency = command(b, &.{"dependency"}, doltlite_lib, cache_root, output_root, go_exe);
    const dependency_step = b.step("doltlite", "Prepare the checksum-verified DoltLite release library");
    dependency_step.dependOn(&dependency.step);

    var binary_runs: [binaries.len]*std.Build.Step.Run = undefined;
    for (binaries, 0..) |binary, i| {
        binary_runs[i] = command(
            b,
            &.{ "go-binary", binary.package, binary.name },
            doltlite_lib,
            cache_root,
            output_root,
            go_exe,
        );
        binary_runs[i].step.dependOn(&dependency.step);
    }

    const provenance = command(b, &.{"provenance"}, doltlite_lib, cache_root, output_root, go_exe);
    for (binary_runs) |run| provenance.step.dependOn(&run.step);

    const plugins_step = b.step("plugins", "Build all DoltLite backend plugin binaries and clients");
    plugins_step.dependOn(&provenance.step);
    b.getInstallStep().dependOn(plugins_step);

    const linked_test = command(b, &.{"test"}, doltlite_lib, cache_root, output_root, go_exe);
    linked_test.step.dependOn(&provenance.step);
    const test_step = b.step("test", "Run linked DoltLite plugin regression tests");
    test_step.dependOn(&linked_test.step);
}

fn command(
    b: *std.Build,
    action_args: []const []const u8,
    doltlite_lib: []const u8,
    cache_root: []const u8,
    output_root: []const u8,
    go_exe: []const u8,
) *std.Build.Step.Run {
    const run = b.addSystemCommand(&.{ "bash", "scripts/zig-build.sh" });
    run.addArgs(action_args);
    run.addArgs(&.{
        "--doltlite-lib",
        doltlite_lib,
        "--cache-root",
        cache_root,
        "--output-root",
        output_root,
        "--go",
        go_exe,
    });
    run.setEnvironmentVariable("ZIG_EXE", b.graph.zig_exe);
    run.setCwd(b.path("."));
    return run;
}
