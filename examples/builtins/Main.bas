Attribute VB_Name = "Main"
Option Explicit

Public Function Run() As Long
    Dim words() As String
    Dim repeated As String
    Dim n As Long

    words = Split("A BB CCC")
    n = AscW(Left$(words(2), 1)) - AscW("A")
    repeated = String(2, "x")
    If StrComp(repeated, "xx", vbBinaryCompare) = 0 Then
        n = n + InStr("abc", "b")
    End If
    Run = IIf(n = 4, (&H50 \ 2) + (&HA Mod 3) + 1, 0)
End Function
