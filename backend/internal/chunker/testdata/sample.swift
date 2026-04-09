import Foundation
import UIKit

// TODO: Implement advanced features
/* 
  FIXME: Optimization needed
*/

/// A sample protocol
protocol Playable {
    func play()
}

/// A sample class
public class MediaPlayer: Playable {
    var volume: Double = 0.5
    
    public init(volume: Double) {
        self.volume = volume
    }
    
    public func play() {
        print("Playing at volume \(volume)")
    }
}

struct Settings {
    var theme: String
}

enum Status {
    case active
    case inactive
}

extension MediaPlayer {
    func stop() {
        print("Stopped")
    }
}

func topLevelFunction() {
    print("Hello from top level")
}
