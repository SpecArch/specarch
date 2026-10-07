import Foundation
import SpecArchKit

exit(run(Array(CommandLine.arguments.dropFirst()), stdout: FileSink(.standardOutput), stderr: FileSink(.standardError)))
