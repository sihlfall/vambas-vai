Attribute VB_Name = "Main"
Option Explicit

Private Type Item
    Value As Long
    Text As String
End Type

Public Function Run() As Long
    Dim items() As Item
    On Error GoTo Failed
    ReDim items(1 To 2)
    With items(1)
        .Value = 42
        .Text = "preserved"
    End With
    ReDim Preserve items(1 To 3)
    If Recover() Then Run = items(1).Value
    Exit Function
Failed:
    Run = -1
End Function

Private Function Recover() As Boolean
    On Error GoTo Handled
    Err.Raise vbObjectError + 1, , "demonstration"
    Exit Function
Handled:
    Recover = Err.Number = vbObjectError + 1 And Err.Description = "demonstration"
End Function
