# Demo for the README. Requires VHS: https://github.com/charmbracelet/vhs
# The app must be logged in (a token in config.yaml).

Output demo.gif

Set Shell "powershell"
Set FontSize 15
Set Width 1280
Set Height 800
Set Padding 20
Set TypingSpeed 120ms
Set Theme "Dracula"

# the player starts on its own
Type ".\yatui.exe"
Enter
Sleep 6s

# open a playlist with the mouse-free path
Sleep 1s

# quick search over the open list
Type "/"
Sleep 500ms
Type "night"
Sleep 1200ms
Escape
Sleep 800ms

# equalizer
Type "F9"
Sleep 1200ms
Escape
Sleep 600ms

# my wave settings
Type "F10"
Sleep 1200ms
Escape
Sleep 600ms

# all hotkeys
Type "?"
Sleep 1500ms
Escape
Sleep 500ms
