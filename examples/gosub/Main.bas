Attribute VB_Name = "Main"
Option Explicit

Public Function Main() As Long
    Dim value As Long
    value = 40
    GoSub AddTwo
    Main = value
    Exit Function
AddTwo:
    value = value + 2
    Return
End Function
