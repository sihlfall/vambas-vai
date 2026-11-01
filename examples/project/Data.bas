Attribute VB_Name = "Data"
Option Explicit

Public Total As Long
Public Enabled As Boolean
Public Samples(1 To 2) As Long
Private Const Base As Long = &H28&

Public Function NextValue( _
    Optional ByVal stepValue As Long = 1, _
    Optional ByRef label As String = "default" _
) As Long
    If Not Enabled Then
        Total = Base
        Enabled = True
    End If
    Total = Total + stepValue
    Samples(1) = Total
    label = "private default temporary"
    NextValue = Samples(1)
End Function
