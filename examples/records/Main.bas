Option Explicit

Public Enum Phase
    Pending = 40
    Complete = Pending + 2
End Enum

Public Type State
    Current As Phase
    Label As String
    Samples() As Double
End Type

Public Function Main() As Long
    Dim original As State, snapshot As State
    original.Current = Phase.Pending
    original.Label = "before"
    ReDim original.Samples(1)
    original.Samples(0) = 1.5
    snapshot = original
    Advance original
    Mid$(original.Label, 1, 1) = "X"
    original.Samples(0) = 99
    If snapshot.Label = "before" And snapshot.Samples(0) = 1.5 Then
        Main = original.Current
    End If
End Function

Private Sub Advance(ByRef value As State)
    value.Current = Phase.Complete
End Sub
