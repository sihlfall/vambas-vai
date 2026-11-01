Option Explicit

Private count As Long

Private Sub Class_Initialize()
    count = 40
End Sub

Public Sub Increment()
    count = count + 1
End Sub

Public Property Get Value() As Long
Attribute Value.VB_UserMemId = 0
    Value = count
End Property
