import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/shell_windows.dart';

void main() {
  test("a system program is the one in the system's folder, by name", () {
    final root = Platform.environment['SystemRoot'] ?? r'C:\Windows';
    expect(systemProgram('explorer.exe'), '$root\\explorer.exe');
  });
}
