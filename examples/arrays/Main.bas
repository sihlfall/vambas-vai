Option Explicit
Option Base 1

Public Function Main() As Long
    Dim values() As Long
    ReDim values(2)
    values(1) = 20
    Grow values()
    Bump values(1)
    Main = values(1) + values(3)
End Function

Private Sub Grow(ByRef values() As Long)
    ReDim Preserve values(3)
    values(3) = 21
End Sub

Private Sub Bump(ByRef value As Variant)
    value = value + 1
End Sub
