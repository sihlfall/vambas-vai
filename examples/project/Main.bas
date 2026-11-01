Attribute VB_Name = "Main"
Option Explicit

Public Function Run() As Long
    Dim enabledAsLong As Long
    Data.NextValue
    enabledAsLong = Data.Enabled
    If enabledAsLong <> -1 Then Exit Function
    Run = Data.NextValue()
End Function
