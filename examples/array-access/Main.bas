Option Explicit

Public Function ArraySum(ByRef values() As Long) As Long
    Dim i As Long
    For i = LBound(values) To UBound(values)
        ArraySum = ArraySum + values(i)
    Next i
End Function

Public Sub Fill(ByRef values() As Long)
    Dim i As Long
    For i = LBound(values) To UBound(values)
        values(i) = i
    Next i
End Sub

Public Function Main() As Long
    Dim values() As Long
    ReDim values(1 To 10)
    Fill values
    Main = ArraySum(values)
End Function
