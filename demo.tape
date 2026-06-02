# TerminalReader Demo
# A TUI novel reader with multi-source search

# Output demo.gif
Output demo.mp4

Require ./reader

Set Shell "bash"
Set FontSize 18
Set FontFamily "Maple Mono NF CN"
Set Width 1000
Set Height 650
Set Padding 20
Set Theme "OneDark"
Set WindowBar Colorful
Set TypingSpeed 100ms
Set PlaybackSpeed 1.5

# Start the app
Type "./reader" Sleep 500ms Enter
Sleep 1s

# ============================================
# Scene 1: Bookshelf View
# ============================================
Sleep 2s
Type "?" Sleep 2s
Type "q" Sleep 1s
Tab Sleep 2s
Type "q" Sleep 2s
Type "j" Sleep 800ms
Type "+" Sleep 1s
Type "k" Sleep 1s

# ============================================
# Scene 2: Read
# ============================================
Enter Sleep 1s
Type "j" Sleep 800ms
Type "j" Sleep 800ms
Right Sleep 1s
Type "j" Sleep 800ms
Type "c" Sleep 2s
Type "j" Sleep 800ms
Type "j" Sleep 800ms
Type "/" Sleep 1s
Type "1" Sleep 500ms
Type "0" Sleep 1s
Enter Sleep 1s
Type "q" Sleep 1s
Sleep 1.5s

# ============================================
# Scene 3: Search
# ============================================
Type "s" Sleep 1s
Type@500ms "斗罗大陆" Sleep 1s
Enter Sleep 1s
Type "j" Sleep 700ms
Type "j" Sleep 700ms
Type "k" Sleep 700ms
Type "k" Sleep 1s
Enter Sleep 1s
Type "b" Sleep 2s
Sleep 10s
Type "x" Sleep 2s
Type "r"

# ============================================
# Scene 18: Final view & quit
# ============================================
Sleep 2s
Type "q" Sleep 1s
