Option Explicit

' Compare the SSA and ordinary LLVM compiler panes.
' Add another compiler pane to see both at once.
Public Function SumTo(ByVal limit As Long) As Long
    Dim i As Long
    Dim total As Long

    For i = 1 To limit
        If i Mod 2 = 0 Then
            total = total + i
        Else
            total = total + 1
        End If
    Next i

    SumTo = total
End Function
