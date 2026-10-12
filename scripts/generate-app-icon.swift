// Render the original Kaelix vector mark as the Wails application icon.
// Usage: swift scripts/generate-app-icon.swift apps/desktop/build/appicon.png
import AppKit
let size = 1024
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSColor(red: 28/255, green: 28/255, blue: 28/255, alpha: 1).setFill()
NSBezierPath(roundedRect: NSRect(x: 0, y: 0, width: size, height: size), xRadius: 200, yRadius: 200).fill()
// SVG coordinates mapped to AppKit's upward Y axis, with generous optical padding.
func point(_ x: Double, _ y: Double) -> NSPoint { NSPoint(x: 160+x*11, y: 864-y*11) }
NSColor(red: 251/255, green: 81/255, blue: 2/255, alpha: 1).setFill()
for polygon in [[(8.0,8.0),(20,8),(20,56),(8,56)],[(26.0,29.0),(46,8),(61,8),(38,32),(61,56),(46,56),(26,35)]] {
 let path = NSBezierPath()
 path.move(to: point(polygon[0].0,polygon[0].1))
 for (x,y) in polygon.dropFirst(){path.line(to: point(x,y))}
 path.close();path.fill()
}
NSGraphicsContext.restoreGraphicsState()
let destination = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "apps/desktop/build/appicon.png"
try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: destination))
