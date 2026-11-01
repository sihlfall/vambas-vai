Option Explicit

Public Function Main() As Long
    Dim values(1 To 3) As Long
    Dim i As Long, total As Long, value As Variant

    For i = 1 To 3
        values(i) = i
    Next
    For Each value In values
        total = total + value
    Next

    i = 0
    Do Until i = 3
        i = i + 1
        total = total + 2
    Loop
    While total < 40
        total = total + 4
    Wend

    For i = 1 To 10
        If i = 2 Then GoTo Done
        total = total + 1
    Next
Done:
    Main = total + 1
End Function
