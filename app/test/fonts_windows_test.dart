import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/fonts_windows.dart';

/// `reg query`'s output of a font key, as Windows prints it.
const _machine =
    '\r\n'
    r'HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts'
    '\r\n'
    '    Arial (TrueType)    REG_SZ    arial.ttf\r\n'
    '    Arial Black (TrueType)    REG_SZ    ariblk.ttf\r\n'
    '    Arial Bold Italic (TrueType)    REG_SZ    arialbi.ttf\r\n'
    '    Cambria & Cambria Math (TrueType)    REG_SZ    cambria.ttc\r\n'
    '    Segoe UI Light (TrueType)    REG_SZ    segoeuil.ttf\r\n'
    '    Modern (All res)    REG_SZ    modern.fon\r\n'
    '\r\n';

void main() {
  test("every family is the registry's, its bold and italic faces one "
      'family, a collection each of its fonts', () {
    expect(registryFamilies(_machine), [
      'Arial',
      'Arial Black',
      'Cambria',
      'Cambria Math',
      'Modern',
      'Segoe UI Light',
    ]);
  });

  test("families reads the machine's key and the user's, each once, "
      "and no name reaches the system's font", () async {
    final asked = <List<String>>[];
    final fonts = WindowsFonts(
      run: (program, arguments) async {
        asked.add([program.split(r'\').last, ...arguments]);
        final user = arguments.last.startsWith('HKCU');
        return ProcessResult(
          1,
          0,
          user
              ? '    Fira Code Regular (TrueType)    REG_SZ    x.ttf\r\n'
              : _machine,
          '',
        );
      },
    );
    expect(await fonts.families(), [
      'Arial',
      'Arial Black',
      'Cambria',
      'Cambria Math',
      'Fira Code',
      'Modern',
      'Segoe UI Light',
    ]);
    expect(asked.map((a) => a.take(2).join(' ')), [
      'reg.exe query',
      'reg.exe query',
    ]);
    expect(fonts.system, isNull);
  });

  test('no answer when neither key could be read, a missing user key '
      'no matter', () async {
    final none = WindowsFonts(
      run: (program, arguments) async => ProcessResult(1, 1, '', 'not found'),
    );
    expect(await none.families(), isNull);
    final noUser = WindowsFonts(
      run: (program, arguments) async => arguments.last.startsWith('HKCU')
          ? ProcessResult(1, 1, '', 'not found')
          : ProcessResult(1, 0, _machine, ''),
    );
    expect(await noUser.families(), hasLength(6));
  });
}
