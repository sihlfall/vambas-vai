Option Explicit

Public Function Main() As Long
    Dim samples() As Single
    Dim x As Double, total As Double
    Dim i As Long
    ReDim samples(0 To 3)
    For x = 0.5 To 2 Step 0.5
        samples(i) = CSng(x * 4)
        i = i + 1
    Next
    For i = 0 To 3
        total = total + samples(i)
    Next
    Main = CLng(total * 2 + 2)
End Function
