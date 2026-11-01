Option Explicit

Public Function Main() As Long
    Dim choice As Long
    choice = 3
    Select Case choice
        Case 0, 1
            choice = 1
        Case 2 To 4
            choice = 2
        Case Is >= 5
            choice = 3
        Case Else
            choice = 0
    End Select

    On choice GoTo Unexpected, Answer, Unexpected
Unexpected:
    Main = -1
    Exit Function
Answer:
    Main = 42
End Function
