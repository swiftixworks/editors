import Foundation
import Swiftix
import SwiftixGoRuntime
import SwiftixPackages
import Testing

/// A kernel with the checked-in editors package installed and a shell on a
/// 24×80 pty, driven one host turn at a time like a terminal consumer.
final class EditorSession {
    let loop = EventLoop()
    let kernel: Kernel
    let terminal = PseudoTerminal()
    private var output: [UInt8] = []

    init(files: [String: String] = [:]) throws {
        kernel = Kernel(loop: loop)
        let archive = try loadArchive()
        kernel.spawn("install-editors") { context in
            _ = context.mkdir("/usr")
            _ = context.mkdir("/usr/bin")
            for entry in archive.files where entry.path.hasPrefix("/usr/bin/") {
                let descriptor = context.open(entry.path, create: true, truncate: true)!
                context.write(descriptor, archive.contents(of: entry))
                context.close(descriptor)
                _ = context.chmod(entry.path, mode: FileMode(rawValue: entry.mode))
            }
            for (path, contents) in files {
                let descriptor = context.open(path, create: true, truncate: true)!
                context.write(descriptor, Array(contents.utf8))
                context.close(descriptor)
            }
            context.exit(0)
        }
        loop.runUntilIdle()

        terminal.windowSize = WindowSize(rows: 24, columns: 80)
        terminal.onOutput = { [unowned self] in
            output.append(contentsOf: terminal.readForApp(max: 65_535))
        }
        let commands = CommandRegistry.builtins
        GoExecutableLoader.register(in: commands)
        kernel.spawn("sh", Programs.shell(tty: terminal.slave, commands: commands))
        loop.runUntilIdle()
        _ = takeOutput()
    }

    /// Send `text` as one terminal write, as a keyboard or paste would.
    func type(_ text: String) {
        send(Array(text.utf8))
    }

    func send(_ bytes: [UInt8]) {
        terminal.writeFromApp(bytes)
        loop.runUntilIdle()
    }

    /// Send each key as its own terminal write.
    func keys(_ sequence: [[UInt8]]) {
        for key in sequence { send(key) }
    }

    /// Terminal output since the previous call.
    func takeOutput() -> String {
        defer { output.removeAll() }
        return String(decoding: output, as: UTF8.self)
    }

    /// The status-line message nano shows for the cursor position (^C).
    func location() -> String? {
        _ = takeOutput()
        send(Key.ctrl("C"))
        return statusMessage(in: takeOutput())
    }

    func contents(of path: String) -> String? {
        var result: String?
        kernel.spawn("read") { context in
            if let descriptor = context.open(path) {
                var bytes: [UInt8] = []
                while true {
                    let chunk = context.read(descriptor, max: 65_536)
                    if chunk.isEmpty { break }
                    bytes.append(contentsOf: chunk)
                }
                context.close(descriptor)
                result = String(decoding: bytes, as: UTF8.self)
            }
            context.exit(0)
        }
        loop.runUntilIdle()
        return result
    }

    private func loadArchive() throws -> PackageArchive {
        let root = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let bytes = Array(try Data(contentsOf: root
            .appendingPathComponent("Artifacts/editors_0.1.0.pkg")))
        return try PackageArchive.decode(bytes)
    }
}

/// Terminal byte sequences for keys.
enum Key {
    static let up: [UInt8] = [0x1B, 0x5B, 0x41]
    static let down: [UInt8] = [0x1B, 0x5B, 0x42]
    static let right: [UInt8] = [0x1B, 0x5B, 0x43]
    static let left: [UInt8] = [0x1B, 0x5B, 0x44]
    static let home: [UInt8] = [0x1B, 0x5B, 0x48]
    static let end: [UInt8] = [0x1B, 0x5B, 0x46]
    static let delete: [UInt8] = [0x1B, 0x5B, 0x33, 0x7E]
    static let pageDown: [UInt8] = [0x1B, 0x5B, 0x36, 0x7E]
    static let escape: [UInt8] = [0x1B]
    static let backspace: [UInt8] = [0x7F]
    static let enter: [UInt8] = [0x0D]

    static func ctrl(_ letter: Character) -> [UInt8] {
        [UInt8(letter.asciiValue! - 64)]
    }
}

/// The text of the last `[ message ]` in a frame.
func statusMessage(in frame: String) -> String? {
    guard let open = frame.range(of: "[ ", options: .backwards),
        let close = frame.range(of: " ]", range: open.upperBound..<frame.endIndex)
    else { return nil }
    return String(frame[open.upperBound..<close.lowerBound])
}

@Suite("nano")
struct NanoTests {

    @Test("checked-in archive contains the svm64 editor")
    func archiveContents() throws {
        let root = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let archive = try PackageArchive.decode(Array(try Data(contentsOf: root
            .appendingPathComponent("Artifacts/editors_0.1.0.pkg"))))

        #expect(archive.manifest.name == "editors")
        #expect(archive.manifest.version.description == "0.1.0")
        #expect(archive.manifest.architecture == "svm64")
        #expect(Set(archive.files.map(\.path)) == [
            "/usr/bin/nano",
            "/usr/share/doc/editors/LICENSE",
            "/usr/share/doc/editors/README.md",
        ])
        let nano = try #require(archive.files.first { $0.path == "/usr/bin/nano" })
        #expect(nano.mode == 0o755)
        #expect(GoExecutableImage.recognizes(archive.contents(of: nano)))
    }

    @Test func createsEditsAndSavesANewFile() throws {
        let session = try EditorSession()

        session.type("nano /notes.txt\n")
        let firstFrame = session.takeOutput()
        #expect(firstFrame.contains("\u{1B}[?1049h"))
        #expect(firstFrame.contains("Swiftix nano 0.1.0"))
        #expect(firstFrame.contains("/notes.txt"))
        #expect(statusMessage(in: firstFrame) == "New File")
        #expect(session.terminal.rawMode)

        session.type("hello")
        session.send(Key.enter)
        session.type("wörld")
        #expect(session.takeOutput().contains("Modified"))

        session.send(Key.ctrl("O"))
        #expect(session.takeOutput().contains("File Name to Write: /notes.txt"))
        session.send(Key.enter)
        #expect(statusMessage(in: session.takeOutput()) == "Wrote 2 lines")

        session.send(Key.ctrl("X"))
        let exitOutput = session.takeOutput()
        #expect(exitOutput.contains("\u{1B}[?1049l"))
        #expect(!session.terminal.rawMode)
        #expect(session.contents(of: "/notes.txt") == "hello\nwörld\n")

        session.type("echo status=$?\n")
        #expect(session.takeOutput().contains("status=0"))
    }

    @Test func cursorKeysKeepTheWantedColumn() throws {
        let session = try EditorSession(files: ["/text": "one\nsix\nthirteen\n"])
        session.type("nano /text\n")
        #expect(statusMessage(in: session.takeOutput()) == "Read 3 lines")

        session.keys([Key.down, Key.down, Key.end])
        #expect(session.location() == "line 3/3, col 9/9")
        session.send(Key.up)
        #expect(session.location() == "line 2/3, col 4/4")
        session.send(Key.down)
        #expect(session.location() == "line 3/3, col 9/9")
        session.keys([Key.home, Key.right, Key.right])
        #expect(session.location() == "line 3/3, col 3/9")
        session.keys([Key.ctrl("E"), Key.right])
        #expect(session.location() == "line 3/3, col 9/9")
        session.keys([Key.ctrl("A"), Key.left])
        #expect(session.location() == "line 2/3, col 4/4")
    }

    @Test func deletionJoinsLinesAndRemovesWholeCharacters() throws {
        let session = try EditorSession(files: ["/text": "añb\nsecond\n"])
        session.type("nano /text\n")

        session.keys([Key.right, Key.right, Key.backspace])
        #expect(session.location() == "line 1/2, col 2/3")
        session.keys([Key.end, Key.delete])
        #expect(session.location() == "line 1/1, col 3/9")
        session.keys([Key.home, Key.ctrl("D")])
        session.send(Key.down)
        session.send(Key.backspace)

        session.send(Key.ctrl("O"))
        session.send(Key.enter)
        session.send(Key.ctrl("X"))
        #expect(session.contents(of: "/text") == "bsecond\n")
    }

    @Test func exitAsksBeforeDiscardingChanges() throws {
        let session = try EditorSession(files: ["/keep": "original\n", "/save": "original\n"])

        session.type("nano /keep\n")
        session.type("changed ")
        session.send(Key.ctrl("X"))
        #expect(session.takeOutput().contains("Save modified buffer? (Y/N/^C)"))
        session.send(Key.ctrl("C"))
        #expect(statusMessage(in: session.takeOutput()) == "Cancelled")
        session.send(Key.ctrl("X"))
        session.type("n")
        #expect(!session.terminal.rawMode)
        #expect(session.contents(of: "/keep") == "original\n")

        session.type("nano /save\n")
        session.type("changed ")
        session.send(Key.ctrl("X"))
        session.type("y")
        #expect(session.takeOutput().contains("File Name to Write: /save"))
        session.send(Key.enter)
        #expect(!session.terminal.rawMode)
        #expect(session.contents(of: "/save") == "changed original\n")
    }

    @Test func consecutiveCutsPasteTogether() throws {
        let session = try EditorSession(files: ["/list": "a\nb\nc\nd\n"])
        session.type("nano /list\n")

        session.keys([Key.ctrl("K"), Key.ctrl("K"), Key.down, Key.ctrl("U")])
        session.send(Key.ctrl("O"))
        session.send(Key.enter)
        session.send(Key.ctrl("X"))

        #expect(session.contents(of: "/list") == "c\na\nb\nd\n")
    }

    @Test func searchFindsWrapsAndRepeats() throws {
        let session = try EditorSession(files: ["/words": "alpha\nbeta alpha\ngamma\n"])
        session.type("nano /words\n")

        session.send(Key.ctrl("W"))
        #expect(session.takeOutput().contains("Search: "))
        session.type("alpha")
        session.send(Key.enter)
        #expect(session.location() == "line 2/3, col 6/11")

        session.send(Key.ctrl("W"))
        #expect(session.takeOutput().contains("Search [alpha]: "))
        session.send(Key.enter)
        #expect(statusMessage(in: session.takeOutput()) == "Search Wrapped")
        #expect(session.location() == "line 1/3, col 1/6")

        session.send(Key.ctrl("W"))
        session.type("gamma")
        session.send(Key.enter)
        session.send(Key.ctrl("W"))
        session.send(Key.enter)
        #expect(statusMessage(in: session.takeOutput()) == "This is the only occurrence")

        session.send(Key.ctrl("W"))
        session.type("delta")
        session.send(Key.enter)
        #expect(statusMessage(in: session.takeOutput()) == "\"delta\" not found")
        session.send(Key.ctrl("W"))
        session.send(Key.escape)
        #expect(statusMessage(in: session.takeOutput()) == "Cancelled")
    }

    @Test func tabsAndControlBytesRenderInTheirColumns() throws {
        let session = try EditorSession(files: ["/tabs": "a\tb\r\n"])
        session.type("nano /tabs\n")
        let frame = session.takeOutput()

        #expect(frame.contains("a       b^M"))
        session.keys([Key.right, Key.right])
        #expect(session.location() == "line 1/1, col 3/5")
    }

    @Test func pagingScrollsTheEditArea() throws {
        let text = (1...50).map { "line \($0)" }.joined(separator: "\n") + "\n"
        let session = try EditorSession(files: ["/long": text])
        session.type("nano /long\n")
        #expect(!session.takeOutput().contains("line 21"))

        session.send(Key.pageDown)
        #expect(session.takeOutput().contains("line 21"))
        #expect(session.location() == "line 21/50, col 1/8")
        session.send(Key.ctrl("V"))
        session.send(Key.ctrl("V"))
        #expect(session.location() == "line 50/50, col 1/8")
    }

    @Test func frameFollowsTheWindowSize() throws {
        let session = try EditorSession()
        session.type("nano\n")
        #expect(session.takeOutput().contains("\u{1B}[24;1H"))

        session.terminal.windowSize = WindowSize(rows: 10, columns: 40)
        session.send(Key.ctrl("L"))
        let frame = session.takeOutput()
        #expect(frame.contains("\u{1B}[10;1H"))
        #expect(!frame.contains("\u{1B}[24;1H"))

        session.terminal.windowSize = WindowSize(rows: 2, columns: 8)
        session.type("still editing")
        #expect(session.takeOutput().contains("\u{1B}[5;1H"))
        #expect(session.terminal.rawMode)
    }

    @Test func largestAcceptedFileLoadsSearchesAndSaves() throws {
        var text = ""
        while text.utf8.count + 64 <= 24_576 {
            text += String(repeating: "x", count: 63) + "\n"
        }
        text = String(text.dropLast(7)) + "needle\n"
        let session = try EditorSession(files: ["/large": text])

        session.type("nano /large\n")
        #expect(statusMessage(in: session.takeOutput()) == "Read 384 lines")
        session.send(Key.ctrl("W"))
        session.type("needle")
        session.send(Key.enter)
        #expect(session.location()?.hasPrefix("line 384/384") == true)
        session.type("!")
        session.send(Key.ctrl("O"))
        session.send(Key.enter)
        session.send(Key.ctrl("X"))

        let saved = try #require(session.contents(of: "/large"))
        #expect(saved.hasSuffix("!needle\n"))
        #expect(saved.utf8.count == text.utf8.count + 1)
    }

    @Test func refusesFilesAboveTheLimit() throws {
        let text = String(repeating: String(repeating: "y", count: 99) + "\n", count: 250)
        let session = try EditorSession(files: ["/huge": text])

        session.type("nano /huge\n")

        #expect(statusMessage(in: session.takeOutput())
            == "File is too large to open (limit 24 KiB)")
    }

    @Test func requiresATerminal() throws {
        let session = try EditorSession(files: ["/input": "x\n"])

        session.type("nano /input < /input\n")
        #expect(session.takeOutput().contains("nano: standard input is not a terminal"))
        session.type("echo status=$?\n")
        #expect(session.takeOutput().contains("status=1"))
    }
}
