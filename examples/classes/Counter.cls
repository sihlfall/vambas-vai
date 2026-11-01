Option Explicit

Private count As Long

Public Sub Increment()
    count = count + 1
End Sub

Public Property Get Value() As Long
    Value = count
End Property
