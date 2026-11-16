import 'package:flutter_test/flutter_test.dart';
import 'package:nushi/conventions.dart';

// The words a phone uses for its paired computer, by the system its daemon
// says it runs on (PingResponse.os, as Go names it).

void main() {
  test("a daemon's system gives its conventions; one that doesn't say is "
      "the Mac's", () {
    expect(conventionsOf('darwin'), same(macConventions));
    expect(conventionsOf('linux'), same(linuxConventions));
    expect(conventionsOf('windows'), same(windowsConventions));
    expect(conventionsOf(''), same(macConventions));
  });
}
