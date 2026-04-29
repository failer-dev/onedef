import 'dart:convert';
import 'dart:io';

import 'package:onedef_gen/onedef_dart_generator.dart';

Future<void> main(List<String> args) async {
  if (args.isEmpty || args.first != 'generate') {
    stderr.writeln(
      'Usage: dart run bin/onedef_dart_generator.dart generate --input <spec.json> --out <dir> --package-name <name> --core-path <relative-path>',
    );
    exitCode = 64;
    return;
  }

  final parsed = _parseFlags(args.skip(1).toList());
  final input = parsed['input'];
  final out = parsed['out'];
  final packageName = parsed['package-name'];
  final corePath = parsed['core-path'];

  if (input == null || out == null || packageName == null || corePath == null) {
    stderr.writeln(
      '--input, --out, --package-name, and --core-path are required',
    );
    exitCode = 64;
    return;
  }

  final jsonText = await File(input).readAsString();
  final spec = Spec.fromJson(
    jsonDecode(jsonText) as Map<String, dynamic>,
  );

  await writePackage(
    spec: spec,
    packageName: packageName,
    outputDir: out,
    corePath: corePath,
  );
}

Map<String, String> _parseFlags(List<String> args) {
  final result = <String, String>{};
  for (var i = 0; i < args.length; i++) {
    final arg = args[i];
    if (!arg.startsWith('--')) {
      continue;
    }
    if (i + 1 >= args.length) {
      break;
    }
    result[arg.substring(2)] = args[i + 1];
    i++;
  }
  return result;
}
