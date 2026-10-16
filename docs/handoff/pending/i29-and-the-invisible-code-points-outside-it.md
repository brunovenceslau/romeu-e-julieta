# I29 and the invisible code points outside it

The security audit of T007 (2026-10-16) found that termsafe keeps some
code points that I29 does not name:

- U+00AD and U+034F
- U+2061 to U+2064 and U+206A to U+206F
- the variation selectors
- the Hangul fillers
- U+FFF9 to U+FFFB

They cannot move the cursor or reorder a line, but they can hide content
in a gate diff.

Widening the set is a change to I29 first. One option is the Unicode
Default_Ignorable_Code_Point property.

Recommended: widen I29 to Default_Ignorable_Code_Point before the gate
diff of M1 prints agent text.

Reopen: the operator decides, or the first gate diff task starts.
